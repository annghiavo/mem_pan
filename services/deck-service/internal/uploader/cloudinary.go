package uploader

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type ImageUploader interface {
	Upload(ctx context.Context, file multipart.File, folder string) (string, error)
}

type cloudinaryUploader struct {
	cld *cloudinary.Cloudinary
}

func NewCloudinary(cloudinaryURL string) (ImageUploader, error) {
	cld, err := cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		return nil, fmt.Errorf("cloudinary init: %w", err)
	}
	return &cloudinaryUploader{cld: cld}, nil
}

func (u *cloudinaryUploader) Upload(ctx context.Context, file multipart.File, folder string) (string, error) {
	resp, err := u.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder: folder,
	})
	if err != nil {
		return "", fmt.Errorf("cloudinary upload: %w", err)
	}
	// Cloudinary SDK does not return a Go error for API-level failures
	// (e.g. missing permissions). The error is buried in resp.Error.Message.
	if resp.Error.Message != "" {
		return "", fmt.Errorf("cloudinary API error: %s", resp.Error.Message)
	}
	url := resp.SecureURL
	if url == "" {
		url = resp.URL
	}
	if url == "" {
		return "", fmt.Errorf("cloudinary returned empty URL (PublicID=%s)", resp.PublicID)
	}
	return url, nil
}

