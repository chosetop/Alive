package contentworldtest

import (
	"context"
	"sort"
	"time"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
)

type Store struct {
	Settings map[contentworld.Key]contentworld.Setting

	ListOpenReturn []contentworld.Setting
	ListAllReturn  []contentworld.Setting

	FailListOpen error
	FailListAll  error
	FailGet      error
	FailUpdate   error

	GetCalls      int
	LastGetKey    contentworld.Key
	UpdateCalls   int
	LastUpdateKey contentworld.Key
	LastUpdate    contentworld.UpdateInput
}

func NewStore() *Store {
	now := time.Unix(100, 0).UTC()
	return &Store{
		Settings: map[contentworld.Key]contentworld.Setting{
			contentworld.Journal: {
				World:       contentworld.Journal,
				Status:      contentworld.Open,
				NavLabel:    "日志",
				SortOrder:   10,
				DefaultView: contentworld.ViewNone,
				Revision:    1,
				UpdatedAt:   now,
			},
			contentworld.Saying: {
				World:       contentworld.Saying,
				Status:      contentworld.Unopened,
				NavLabel:    "片语",
				SortOrder:   20,
				DefaultView: contentworld.ViewStream,
				Revision:    1,
				UpdatedAt:   now,
			},
			contentworld.Video: {
				World:       contentworld.Video,
				Status:      contentworld.Unopened,
				NavLabel:    "影像",
				SortOrder:   30,
				DefaultView: contentworld.ViewNone,
				Revision:    1,
				UpdatedAt:   now,
			},
		},
	}
}

func (s *Store) ListOpen(context.Context) ([]contentworld.Setting, error) {
	if s.FailListOpen != nil {
		return nil, s.FailListOpen
	}
	if s.ListOpenReturn != nil {
		return cloneList(s.ListOpenReturn), nil
	}

	worlds := make([]contentworld.Setting, 0, len(s.Settings))
	for _, setting := range s.Settings {
		if setting.Status == contentworld.Open {
			worlds = append(worlds, setting)
		}
	}
	sortSettings(worlds)
	return worlds, nil
}

func (s *Store) ListAll(context.Context) ([]contentworld.Setting, error) {
	if s.FailListAll != nil {
		return nil, s.FailListAll
	}
	if s.ListAllReturn != nil {
		return cloneList(s.ListAllReturn), nil
	}

	worlds := make([]contentworld.Setting, 0, len(s.Settings))
	for _, setting := range s.Settings {
		worlds = append(worlds, setting)
	}
	sortSettings(worlds)
	return worlds, nil
}

func (s *Store) Get(_ context.Context, key contentworld.Key) (contentworld.Setting, error) {
	s.GetCalls++
	s.LastGetKey = key

	if s.FailGet != nil {
		return contentworld.Setting{}, s.FailGet
	}

	setting, ok := s.Settings[key]
	if !ok {
		return contentworld.Setting{}, contentworld.ErrWorldNotFound
	}
	return setting, nil
}

func (s *Store) Update(_ context.Context, key contentworld.Key, input contentworld.UpdateInput) (contentworld.Setting, error) {
	s.UpdateCalls++
	s.LastUpdateKey = key
	s.LastUpdate = input

	if s.FailUpdate != nil {
		return contentworld.Setting{}, s.FailUpdate
	}

	setting, ok := s.Settings[key]
	if !ok {
		return contentworld.Setting{}, contentworld.ErrWorldNotFound
	}
	if setting.Revision != input.ExpectedRevision {
		return contentworld.Setting{}, contentworld.ErrVersionConflict
	}

	if input.Status != nil {
		setting.Status = *input.Status
	}
	if input.NavLabel != nil {
		setting.NavLabel = *input.NavLabel
	}
	if input.DefaultView != nil {
		setting.DefaultView = *input.DefaultView
	}
	setting.Revision++
	setting.UpdatedAt = setting.UpdatedAt.Add(time.Second)
	s.Settings[key] = setting
	return setting, nil
}

func sortSettings(worlds []contentworld.Setting) {
	sort.Slice(worlds, func(i, j int) bool {
		if worlds[i].SortOrder == worlds[j].SortOrder {
			return worlds[i].World < worlds[j].World
		}
		return worlds[i].SortOrder < worlds[j].SortOrder
	})
}

func cloneList(in []contentworld.Setting) []contentworld.Setting {
	out := make([]contentworld.Setting, len(in))
	copy(out, in)
	return out
}
