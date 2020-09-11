package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/boundary/internal/db"
	"github.com/hashicorp/boundary/internal/oplog"
	"github.com/hashicorp/boundary/internal/session/store"
	"google.golang.org/protobuf/proto"
)

const (
	defaultSessionTableName = "session"
)

// ComposedOf defines the boundary data that is referenced to compose a session.
type ComposedOf struct {
	// UserId of the session
	UserId string
	// HostId of the session
	HostId string
	// TargetId of the session
	TargetId string
	// HostSetId of the session
	HostSetId string
	// AuthTokenId of the session
	AuthTokenId string
	// ScopeId of the session
	ScopeId string
}

// Session contains information about a user's session with a target
type Session struct {
	*store.Session
	tableName string `gorm:"-"`
}

var _ Cloneable = (*Session)(nil)
var _ db.VetForWriter = (*Session)(nil)

// New creates a new in memory session.  No options
// are currently supported.
func New(c ComposedOf, opt ...Option) (*Session, error) {
	s := Session{
		Session: &store.Session{
			UserId:      c.UserId,
			HostId:      c.HostId,
			TargetId:    c.TargetId,
			SetId:       c.HostSetId,
			AuthTokenId: c.AuthTokenId,
			ScopeId:     c.ScopeId,
		},
	}

	if err := s.validateNewSession("new session:"); err != nil {
		return nil, err
	}
	return &s, nil
}

// AllocSession will allocate a Session
func AllocSession() Session {
	return Session{
		Session: &store.Session{},
	}
}

// Clone creates a clone of the Session
func (s *Session) Clone() interface{} {
	cp := proto.Clone(s.Session)
	return &Session{
		Session: cp.(*store.Session),
	}
}

// VetForWrite implements db.VetForWrite() interface and validates the session
// before it's written.
func (s *Session) VetForWrite(ctx context.Context, r db.Reader, opType db.OpType, opt ...db.Option) error {
	opts := db.GetOpts(opt...)
	if s.PublicId == "" {
		return fmt.Errorf("session vet for write: missing public id: %w", db.ErrInvalidParameter)
	}
	switch opType {
	case db.CreateOp:
		if err := s.validateNewSession("session vet for write:"); err != nil {
			return err
		}
	case db.UpdateOp:
		switch {
		case contains(opts.WithFieldMaskPaths, "PublicId"):
			return fmt.Errorf("session vet for write: public id is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "UserId"):
			return fmt.Errorf("session vet for write: user id is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "HostId"):
			return fmt.Errorf("session vet for write: host id is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "TargetId"):
			return fmt.Errorf("session vet for write: target id is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "SetId"):
			return fmt.Errorf("session vet for write: set id is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "AuthTokenId"):
			return fmt.Errorf("session vet for write: auth token id is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "CreateTime"):
			return fmt.Errorf("session vet for write: create time is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "UpdateTime"):
			return fmt.Errorf("session vet for write: update time is immutable: %w", db.ErrInvalidParameter)
		case contains(opts.WithFieldMaskPaths, "TerminationReason"):
			if _, err := convertToReason(s.TerminationReason); err != nil {
				return fmt.Errorf("session vet for write: %w", db.ErrInvalidParameter)
			}
		}
	}
	return nil
}

// TableName returns the tablename to override the default gorm table name
func (s *Session) TableName() string {
	if s.tableName != "" {
		return s.tableName
	}
	return defaultSessionTableName
}

// SetTableName sets the tablename and satisfies the ReplayableMessage
// interface. If the caller attempts to set the name to "" the name will be
// reset to the default name.
func (s *Session) SetTableName(n string) {
	s.tableName = n
}

// validateNewSession checks everything but the session's PublicId
func (s *Session) validateNewSession(errorPrefix string) error {
	if s.UserId == "" {
		return fmt.Errorf("%s missing user id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.HostId == "" {
		return fmt.Errorf("%s missing host id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.TargetId == "" {
		return fmt.Errorf("%s missing target id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.SetId == "" {
		return fmt.Errorf("%s missing host set id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.AuthTokenId == "" {
		return fmt.Errorf("%s missing auth token id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.ScopeId == "" {
		return fmt.Errorf("%s missing scope id: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.TerminationReason != "" {
		return fmt.Errorf("%s termination reason must be empty: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.ServerId != "" {
		return fmt.Errorf("%s server id must be empty: %w", errorPrefix, db.ErrInvalidParameter)
	}
	if s.ServerType != "" {
		return fmt.Errorf("%s server type must be empty: %w", errorPrefix, db.ErrInvalidParameter)
	}
	return nil
}

func contains(ss []string, t string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, t) {
			return true
		}
	}
	return false
}

func (s *Session) oplog(op oplog.OpType) oplog.Metadata {
	metadata := oplog.Metadata{
		"resource-public-id": []string{s.PublicId},
		"resource-type":      []string{"session"},
		"op-type":            []string{op.String()},
		"scope-id":           []string{s.ScopeId},
	}
	return metadata
}