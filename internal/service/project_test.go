package service_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/seed"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestCreateAndGetProject(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateProject(ctx, service.CreateProjectInput{
		Name: "Apollo",
		Key:  "apollo",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if created.Name != "Apollo" {
		t.Fatalf("name = %q", created.Name)
	}
	if created.Key != "apollo" {
		t.Fatalf("key = %q want apollo", created.Key)
	}
	if created.Color != "" {
		t.Fatalf("color = %q want empty (optional)", created.Color)
	}
	if created.Archived {
		t.Fatal("new project should not be archived")
	}

	got, err := s.GetProject(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.ID != created.ID || got.Name != "Apollo" || got.Key != "apollo" {
		t.Fatalf("GetProject = %+v", got)
	}
}

func TestCreateProjectDefaultsKeyToName(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateProject(ctx, service.CreateProjectInput{
		Name: "Client Work",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if created.Key != "Client Work" {
		t.Fatalf("key = %q want Client Work", created.Key)
	}
}

func TestCreateProjectWithOptionalColor(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateProject(ctx, service.CreateProjectInput{
		Name:  "Design",
		Color: "#8B5CF6",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if created.Color != "#8B5CF6" {
		t.Fatalf("color = %q want #8B5CF6", created.Color)
	}

	_, err = s.CreateProject(ctx, service.CreateProjectInput{
		Name:  "Bad Color",
		Color: "#bad",
	})
	if err == nil {
		t.Fatal("expected invalid color to fail")
	}
}

func TestUpdateProject(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateProject(ctx, service.CreateProjectInput{
		Name: "Apollo",
		Key:  "apollo",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	updated, err := s.UpdateProject(ctx, service.UpdateProjectInput{
		ID:    created.ID,
		Name:  "Apollo v2",
		Key:   "apollo-v2",
		Color: "#10B981",
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.Name != "Apollo v2" || updated.Key != "apollo-v2" || updated.Color != "#10B981" {
		t.Fatalf("updated = %+v", updated)
	}
}

func TestArchiveProjectHidesFromListButRemainsGettable(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateProject(ctx, service.CreateProjectInput{
		Name: "Legacy",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	archived, err := s.ArchiveProject(ctx, created.ID)
	if err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	if !archived.Archived {
		t.Fatal("expected archived=true")
	}

	active, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for _, p := range active {
		if p.ID == created.ID {
			t.Fatalf("archived project %d still in active list", created.ID)
		}
	}

	got, err := s.GetProject(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if !got.Archived || got.Name != "Legacy" {
		t.Fatalf("GetProject = %+v", got)
	}
}

func TestDeleteProjectRemovesUnused(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	created, err := s.CreateProject(ctx, service.CreateProjectInput{
		Name: "Temporary",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.DeleteProject(ctx, created.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	if _, err := s.GetProject(ctx, created.ID); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteProjectBlocksReferenced(t *testing.T) {
	svc, conn := newSvcConn(t)
	ctx := context.Background()

	created, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Name: "In Use",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	periods, err := svc.ListPeriods(ctx)
	if err != nil || len(periods) == 0 {
		t.Fatalf("ListPeriods: %v", err)
	}
	entry, err := svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     periods[0].ID,
		Day:          periods[0].StartDate,
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
	})
	if err != nil {
		t.Fatalf("CreateTimeEntry: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE time_entry SET project_id = ? WHERE id = ?`, created.ID, entry.ID); err != nil {
		t.Fatalf("attach project: %v", err)
	}

	err = svc.DeleteProject(ctx, created.ID)
	if err == nil {
		t.Fatal("expected error deleting referenced project")
	}
	if !errors.Is(err, service.ErrProjectInUse) {
		t.Fatalf("expected ErrProjectInUse, got %v", err)
	}
}

func TestListAllProjectsMarksArchivedAndInUse(t *testing.T) {
	svc, conn := newSvcConn(t)
	ctx := context.Background()

	created, err := svc.CreateProject(ctx, service.CreateProjectInput{
		Name: "Billable",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	periods, err := svc.ListPeriods(ctx)
	if err != nil || len(periods) == 0 {
		t.Fatalf("ListPeriods: %v", err)
	}
	entry, err := svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     periods[0].ID,
		Day:          periods[0].StartDate,
		StartMinutes: 11 * 60,
		EndMinutes:   12 * 60,
	})
	if err != nil {
		t.Fatalf("CreateTimeEntry: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE time_entry SET project_id = ? WHERE id = ?`, created.ID, entry.ID); err != nil {
		t.Fatalf("attach project: %v", err)
	}

	archived, err := svc.ArchiveProject(ctx, created.ID)
	if err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	if !archived.Archived {
		t.Fatal("expected archived=true")
	}

	all, err := svc.ListAllProjects(ctx)
	if err != nil {
		t.Fatalf("ListAllProjects: %v", err)
	}
	found := false
	for _, p := range all {
		if p.ID == created.ID {
			found = true
			if !p.Archived {
				t.Fatal("ListAllProjects should mark archived")
			}
			if !p.InUse {
				t.Fatal("ListAllProjects should mark in-use when referenced")
			}
		}
	}
	if !found {
		t.Fatal("archived project missing from ListAllProjects")
	}
}

func newSvcConn(t *testing.T) (*service.Service, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := seed.Dev(context.Background(), conn); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return service.New(conn), conn
}
