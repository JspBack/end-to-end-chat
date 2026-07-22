package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Profile struct {
	Name       string `json:"name"`
	ProfilePic string `json:"profile_pic"`
}

type ProfileStore struct {
	db *sql.DB
}

func (p *ProfileStore) Get() (*Profile, error) {
	var prof Profile
	q := "SELECT COALESCE(name, ''), COALESCE(profile_pic, '') FROM profile WHERE id = 1"
	err := p.db.QueryRowContext(context.Background(), q).Scan(&prof.Name, &prof.ProfilePic)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &Profile{}, nil
		}
		return nil, fmt.Errorf("store: get profile: %w", err)
	}
	return &prof, nil
}

func (p *ProfileStore) SetName(name string) error {
	q := "INSERT OR REPLACE INTO profile (id, name, profile_pic) VALUES (1, ?, COALESCE((SELECT profile_pic FROM profile WHERE id = 1), ''))"
	if _, err := p.db.ExecContext(context.Background(), q, name); err != nil {
		return fmt.Errorf("store: set profile name: %w", err)
	}
	return nil
}

func (p *ProfileStore) SetProfilePic(fileID uuid.UUID) error {
	idStr := ""
	if fileID != uuid.Nil {
		idStr = fileID.String()
	}
	q := "INSERT OR REPLACE INTO profile (id, name, profile_pic) VALUES (1, COALESCE((SELECT name FROM profile WHERE id = 1), ''), ?)"
	if _, err := p.db.ExecContext(context.Background(), q, idStr); err != nil {
		return fmt.Errorf("store: set profile pic: %w", err)
	}
	return nil
}

func (p *ProfileStore) Set(name string, fileID uuid.UUID) error {
	idStr := ""
	if fileID != uuid.Nil {
		idStr = fileID.String()
	}
	q := "INSERT OR REPLACE INTO profile (id, name, profile_pic) VALUES (1, ?, ?)"
	if _, err := p.db.ExecContext(context.Background(), q, name, idStr); err != nil {
		return fmt.Errorf("store: set profile: %w", err)
	}
	return nil
}
