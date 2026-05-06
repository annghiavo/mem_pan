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
	return resp.SecureURL, nil
}
