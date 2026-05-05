package service

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

const MaxReferenceImageBytes = 10 << 20
const MaxReferenceImages = 4
const MaxResultImageBytes = 64 << 20

var allowedImageExt = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

type ImageMeta struct {
	Path      string
	Width     int
	Height    int
	SizeBytes int64
}

func TaskImageDir(createdAt time.Time, taskID string) string {
	return filepath.ToSlash(filepath.Join("images", createdAt.Format("2006-01"), taskID))
}

func SaveReferenceImage(dataDir string, createdAt time.Time, taskID string, file *multipart.FileHeader) (string, error) {
	return SaveReferenceImageAt(dataDir, createdAt, taskID, 0, file)
}

func SaveReferenceImageAt(dataDir string, createdAt time.Time, taskID string, index int, file *multipart.FileHeader) (string, error) {
	if file.Size > MaxReferenceImageBytes {
		return "", errors.New("reference image exceeds 10MB")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExt[ext] {
		return "", errors.New("reference image must be PNG, JPEG, or WebP")
	}
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, MaxReferenceImageBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxReferenceImageBytes {
		return "", errors.New("reference image exceeds 10MB")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("reference image is not a supported image")
	}
	name := referenceImageName(index, ext)
	rel := filepath.ToSlash(filepath.Join(TaskImageDir(createdAt, taskID), "refs", name))
	abs := filepath.Join(dataDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", err
	}
	return rel, saveThumb(dataDir, createdAt, taskID, thumbNameForReference(name), img)
}

func CopyReferenceImage(dataDir string, sourceRelPath string, createdAt time.Time, taskID string) (string, error) {
	return CopyReferenceImageAt(dataDir, sourceRelPath, createdAt, taskID, 0)
}

func CopyReferenceImageAt(dataDir string, sourceRelPath string, createdAt time.Time, taskID string, index int) (string, error) {
	if sourceRelPath == "" {
		return "", nil
	}
	ext := strings.ToLower(filepath.Ext(sourceRelPath))
	if !allowedImageExt[ext] {
		return "", errors.New("reference image must be PNG, JPEG, or WebP")
	}
	sourceAbs, err := ResolveSafePath(dataDir, sourceRelPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(sourceAbs)
	if err != nil {
		return "", err
	}
	if len(data) > MaxReferenceImageBytes {
		return "", errors.New("reference image exceeds 10MB")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("reference image is not a supported image")
	}
	name := referenceImageName(index, ext)
	rel := filepath.ToSlash(filepath.Join(TaskImageDir(createdAt, taskID), "refs", name))
	abs := filepath.Join(dataDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", err
	}
	return rel, saveThumb(dataDir, createdAt, taskID, thumbNameForReference(name), img)
}

func referenceImageName(index int, ext string) string {
	return "ref-" + strconv.Itoa(index+1) + ext
}

func thumbNameForReference(fileName string) string {
	ext := filepath.Ext(fileName)
	return strings.TrimSuffix(fileName, ext) + ".jpg"
}

func SaveResultFile(dataDir string, createdAt time.Time, taskID string, file *multipart.FileHeader) (ImageMeta, error) {
	if file == nil {
		return ImageMeta{}, errors.New("result image file is required")
	}
	if file.Size > MaxResultImageBytes {
		return ImageMeta{}, errors.New("result image exceeds 64MB")
	}
	src, err := file.Open()
	if err != nil {
		return ImageMeta{}, err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, MaxResultImageBytes+1))
	if err != nil {
		return ImageMeta{}, err
	}
	if len(data) > MaxResultImageBytes {
		return ImageMeta{}, errors.New("result image exceeds 64MB")
	}
	return SaveResultBytes(dataDir, createdAt, taskID, data)
}

func SaveResultBytes(dataDir string, createdAt time.Time, taskID string, data []byte) (ImageMeta, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ImageMeta{}, errors.New("result image is not a supported image")
	}
	rel := filepath.ToSlash(filepath.Join(TaskImageDir(createdAt, taskID), "output", "result.png"))
	abs := filepath.Join(dataDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return ImageMeta{}, err
	}
	if err := imaging.Save(img, abs); err != nil {
		return ImageMeta{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return ImageMeta{}, err
	}
	if err := saveThumb(dataDir, createdAt, taskID, "result.jpg", img); err != nil {
		return ImageMeta{}, err
	}
	bounds := img.Bounds()
	return ImageMeta{Path: rel, Width: bounds.Dx(), Height: bounds.Dy(), SizeBytes: info.Size()}, nil
}

func saveThumb(dataDir string, createdAt time.Time, taskID string, name string, img image.Image) error {
	thumb := imaging.Fit(img, 400, 400, imaging.Lanczos)
	rel := filepath.ToSlash(filepath.Join(TaskImageDir(createdAt, taskID), "thumbs", name))
	abs := filepath.Join(dataDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return imaging.Save(thumb, abs, imaging.JPEGQuality(80))
}

func ResolveSafePath(dataDir, relPath string) (string, error) {
	relPath = strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	if relPath == "" || strings.Contains(relPath, "..") {
		return "", errors.New("invalid file path")
	}
	abs := filepath.Clean(filepath.Join(dataDir, filepath.FromSlash(relPath)))
	root := filepath.Clean(dataDir) + string(os.PathSeparator)
	if !strings.HasPrefix(abs, root) {
		return "", errors.New("invalid file path")
	}
	return abs, nil
}

func ExtractTaskIDFromImagePath(relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "task_") {
			return part
		}
	}
	return ""
}
