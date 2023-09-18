// Package nss implements the nss grpc service protocol to the daemon.
package nss

import (
	"context"
	"errors"

	"github.com/ubuntu/authd"
	"github.com/ubuntu/authd/internal/cache"
	"github.com/ubuntu/authd/internal/log"
)

// Service is the implementation of the NSS module service.
type Service struct {
	cache *cache.Cache
	authd.UnimplementedNSSServer
}

// NewService returns a new NSS GRPC service.
func NewService(ctx context.Context, cache *cache.Cache) Service {
	log.Debug(ctx, "Building new GRPC NSS service")

	return Service{
		cache: cache,
	}
}

// GetPasswdByName returns the passwd entry for the given username.
func (s Service) GetPasswdByName(ctx context.Context, req *authd.GetByNameRequest) (*authd.PasswdEntry, error) {
	if req.GetName() == "" {
		return nil, errors.New("no user name provided")
	}
	u, err := s.cache.UserByName(req.GetName())
	if err != nil {
		return nil, err
	}

	return newPasswdEntryFromUserPasswdShadow(u), nil
}

// GetPasswdByUID returns the passwd entry for the given UID.
func (s Service) GetPasswdByUID(ctx context.Context, req *authd.GetByIDRequest) (*authd.PasswdEntry, error) {
	u, err := s.cache.UserByID(int(req.GetId()))
	if err != nil {
		return nil, err
	}

	return newPasswdEntryFromUserPasswdShadow(u), nil
}

// GetPasswdEntries returns all passwd entries.
func (s Service) GetPasswdEntries(ctx context.Context, req *authd.Empty) (*authd.PasswdEntries, error) {
	allUsers, err := s.cache.AllUsers()
	if err != nil {
		return nil, err
	}

	var r authd.PasswdEntries
	for _, u := range allUsers {
		r.Entries = append(r.Entries, newPasswdEntryFromUserPasswdShadow(u))
	}

	return &r, nil
}

// GetGroupByName returns the group entry for the given group name.
func (s Service) GetGroupByName(ctx context.Context, req *authd.GetByNameRequest) (*authd.GroupEntry, error) {
	if req.GetName() == "" {
		return nil, errors.New("no group name provided")
	}
	g, err := s.cache.GroupByName(req.GetName())
	if err != nil {
		return nil, err
	}

	return newGroupEntryFromGroup(g), nil
}

// GetGroupByGID returns the group entry for the given GID.
func (s Service) GetGroupByGID(ctx context.Context, req *authd.GetByIDRequest) (*authd.GroupEntry, error) {
	g, err := s.cache.GroupByID(int(req.GetId()))
	if err != nil {
		return nil, err
	}

	return newGroupEntryFromGroup(g), nil
}

// GetGroupEntries returns all group entries.
func (s Service) GetGroupEntries(ctx context.Context, req *authd.Empty) (*authd.GroupEntries, error) {
	allGroups, err := s.cache.AllGroups()
	if err != nil {
		return nil, err
	}

	var r authd.GroupEntries
	for _, g := range allGroups {
		r.Entries = append(r.Entries, newGroupEntryFromGroup(g))
	}

	return &r, nil
}

// GetShadowByName returns the shadow entry for the given username.
func (s Service) GetShadowByName(ctx context.Context, req *authd.GetByNameRequest) (*authd.ShadowEntry, error) {
	if req.GetName() == "" {
		return nil, errors.New("no group name provided")
	}
	u, err := s.cache.UserByName(req.GetName())
	if err != nil {
		return nil, err
	}

	return newShadowEntryFromUserPasswdShadow(u), nil
}

// GetShadowEntries returns all shadow entries.
func (s Service) GetShadowEntries(ctx context.Context, req *authd.Empty) (*authd.ShadowEntries, error) {
	allUsers, err := s.cache.AllUsers()
	if err != nil {
		return nil, err
	}

	var r authd.ShadowEntries
	for _, u := range allUsers {
		r.Entries = append(r.Entries, newShadowEntryFromUserPasswdShadow(u))
	}

	return &r, nil
}

// newPasswdEntryFromUserPasswdShadow returns a PasswdEntry from UserPasswdShadow.
func newPasswdEntryFromUserPasswdShadow(u cache.UserPasswdShadow) *authd.PasswdEntry {
	return &authd.PasswdEntry{
		Name:    u.Name,
		Passwd:  "x",
		Uid:     uint32(u.UID),
		Gid:     uint32(u.GID),
		Gecos:   u.Gecos,
		Homedir: u.Dir,
		Shell:   u.Shell,
	}
}

// newGroupEntryFromGroup returns a GroupEntry from a Group.
func newGroupEntryFromGroup(g cache.Group) *authd.GroupEntry {
	return &authd.GroupEntry{
		Name:    g.Name,
		Passwd:  "x",
		Gid:     uint32(g.GID),
		Members: g.Users,
	}
}

// newShadowEntryFromUserPasswdShadow returns a ShadowEntry from UserPasswdShadow.
func newShadowEntryFromUserPasswdShadow(u cache.UserPasswdShadow) *authd.ShadowEntry {
	return &authd.ShadowEntry{
		Name:               u.Name,
		Passwd:             "x",
		LastChange:         int32(u.LastPwdChange),
		ChangeMinDays:      int32(u.MinPwdAge),
		ChangeMaxDays:      int32(u.MaxPwdAge),
		ChangeWarnDays:     int32(u.PwdWarnPeriod),
		ChangeInactiveDays: int32(u.PwdInactivity),
		ExpireDate:         int32(u.ExpirationDate),
	}
}
