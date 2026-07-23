package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Profile struct {
	Name       string    `json:"name"`
	ProfilePic uuid.UUID `json:"profile_pic"`
}

type ProfileStore struct {
	db *sql.DB
}

func (p *ProfileStore) Get() (*Profile, error) {
	var prof Profile
	q := "SELECT COALESCE(name, ''), COALESCE(profile_pic, '') FROM profile WHERE rowid = 1"
	err := p.db.QueryRowContext(context.Background(), q).Scan(&prof.Name, &prof.ProfilePic)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &Profile{}, nil
		}
		return nil, fmt.Errorf("store: get profile: %w", err)
	}
	return &prof, nil
}

func (p *ProfileStore) Update(name string, fileID uuid.UUID) error {
	_, _ = p.db.ExecContext(context.Background(), "INSERT OR IGNORE INTO profile (rowid) VALUES (1)")
	idStr := ""
	if fileID != uuid.Nil {
		idStr = fileID.String()
	}
	_, err := p.db.ExecContext(context.Background(), "UPDATE profile SET name = ?, profile_pic = ? WHERE rowid = 1", name, idStr)
	if err != nil {
		return fmt.Errorf("store: set profile: %w", err)
	}
	return nil
}
