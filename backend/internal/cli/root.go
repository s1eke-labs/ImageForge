package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"imageforge/backend/internal/config"
	"imageforge/backend/internal/database"
	"imageforge/backend/internal/model"
	"imageforge/backend/internal/service"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func NewRoot() *cobra.Command {
	root := &cobra.Command{Use: "imageforge-cli"}
	userCmd := &cobra.Command{Use: "user", Short: "Manage users"}
	userCmd.AddCommand(createUserCmd(), listUserCmd(), resetPasswordCmd())
	runnerCmd := &cobra.Command{Use: "runner", Short: "Manage runners"}
	runnerCmd.AddCommand(createRunnerCmd(), listRunnerCmd(), deleteRunnerCmd())
	root.AddCommand(userCmd, runnerCmd)
	return root
}

func loadConfig() (config.Config, error) {
	return config.Load(false)
}

func openDB() (*gorm.DB, error) {
	cfg, err := config.Load(false)
	if err != nil {
		return nil, err
	}
	return database.Open(cfg.DBPath)
}

func createUserCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if username == "" || password == "" {
				return errors.New("username and password are required")
			}
			db, err := openDB()
			if err != nil {
				return err
			}
			var count int64
			if err := db.Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("user %q already exists", username)
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if err := db.Create(&model.User{Username: username, PasswordHash: string(hash), TokenVersion: 1}).Error; err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created user %s\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "username")
	cmd.Flags().StringVar(&password, "password", "", "password")
	return cmd
}

func createRunnerCmd() *cobra.Command {
	var name, url string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a runner",
		Long:  "Create a project runner and print its token once.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return errors.New("name is required")
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if strings.TrimSpace(url) == "" {
				url = "http://localhost:" + cfg.Port + "/api/runner"
			}
			db, err := database.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			creds, err := service.CreateRunner(db, name)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "runner_id: %s\n", creds.RunnerID)
			fmt.Fprintf(out, "name: %s\n", creds.Name)
			fmt.Fprintf(out, "url: %s\n", strings.TrimSpace(url))
			fmt.Fprintf(out, "token: %s\n", creds.Token)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "runner name")
	cmd.Flags().StringVar(&url, "url", "", "runner API URL")
	return cmd
}

func listRunnerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List runners",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			var runners []model.Runner
			if err := db.Order("created_at ASC").Find(&runners).Error; err != nil {
				return err
			}
			now := time.Now()
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "id\tname\tstatus\tversion\tlast_heartbeat_at\tcreated_at")
			for _, runner := range runners {
				fmt.Fprintf(
					out,
					"%s\t%s\t%s\t%s\t%s\t%s\n",
					runner.ID,
					runner.Name,
					runner.EffectiveStatus(now),
					runner.Version,
					formatOptionalTime(runner.LastHeartbeatAt),
					runner.CreatedAt.Format("2006-01-02 15:04:05"),
				)
			}
			return nil
		},
	}
}

func deleteRunnerCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a runner",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				return errors.New("id is required")
			}
			db, err := openDB()
			if err != nil {
				return err
			}
			res := db.Where("id = ?", id).Delete(&model.Runner{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("runner %q not found", id)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted runner %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "runner id")
	return cmd
}

func listUserCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			var users []model.User
			if err := db.Order("created_at ASC").Find(&users).Error; err != nil {
				return err
			}
			for _, user := range users {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", user.ID, user.Username, user.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}
}

func resetPasswordCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Reset a user password",
		RunE: func(cmd *cobra.Command, args []string) error {
			if username == "" || password == "" {
				return errors.New("username and password are required")
			}
			db, err := openDB()
			if err != nil {
				return err
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			res := db.Model(&model.User{}).Where("username = ?", username).Updates(map[string]any{
				"password_hash": string(hash),
				"token_version": gorm.Expr("token_version + ?", 1),
			})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("user %q not found", username)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reset password for %s\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "username")
	cmd.Flags().StringVar(&password, "password", "", "password")
	return cmd
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
}
