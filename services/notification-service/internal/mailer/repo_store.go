package mailer

import (
	"context"

	"mem_pan/services/notification-service/internal/repository"
)

// RepoStore is a TemplateStore backed by NotificationRepository.
type RepoStore struct {
	repo repository.NotificationRepository
}

func NewRepoStore(repo repository.NotificationRepository) *RepoStore {
	return &RepoStore{repo: repo}
}

func (s *RepoStore) Get(ctx context.Context, key, locale string) (Template, error) {
	row, err := s.repo.GetActiveEmailTemplate(ctx, key, locale)
	if err != nil {
		return Template{}, err
	}
	return Template{Subject: row.Subject, HTML: row.HtmlBody, Text: row.TextBody}, nil
}
