package appapi

import (
	"context"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type ProjectService struct{ service *service.Service }

func (s *ProjectService) ListProjects(ctx context.Context, req *connect.Request[appv1.ListProjectsRequest]) (*connect.Response[appv1.ListProjectsResponse], error) {
	var items []service.Project
	var err error
	if req.Msg.IncludeArchived {
		items, err = s.service.ListAllProjects(ctx)
	} else {
		items, err = s.service.ListProjects(ctx)
	}
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.Project, len(items))
	for i := range items {
		out[i] = toProtoProject(items[i])
	}
	return connect.NewResponse(&appv1.ListProjectsResponse{Projects: out}), nil
}

func (s *ProjectService) GetProject(ctx context.Context, req *connect.Request[appv1.GetProjectRequest]) (*connect.Response[appv1.GetProjectResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	item, err := s.service.GetProject(ctx, req.Msg.Id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetProjectResponse{Project: toProtoProject(item)}), nil
}

func (s *ProjectService) CreateProject(ctx context.Context, req *connect.Request[appv1.CreateProjectRequest]) (*connect.Response[appv1.CreateProjectResponse], error) {
	if req.Msg.Name == "" {
		return nil, invalidArgument("name is required")
	}
	if req.Msg.Color != "" {
		if err := service.ValidateCategoryColor(req.Msg.Color); err != nil {
			return nil, invalidArgument("color is invalid")
		}
	}
	item, err := s.service.CreateProject(ctx, service.CreateProjectInput{
		Name:  req.Msg.Name,
		Key:   req.Msg.Key,
		Color: req.Msg.Color,
	})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.CreateProjectResponse{Project: toProtoProject(item)}), nil
}

func (s *ProjectService) UpdateProject(ctx context.Context, req *connect.Request[appv1.UpdateProjectRequest]) (*connect.Response[appv1.UpdateProjectResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if req.Msg.Name == "" {
		return nil, invalidArgument("name is required")
	}
	if req.Msg.Color != "" {
		if err := service.ValidateCategoryColor(req.Msg.Color); err != nil {
			return nil, invalidArgument("color is invalid")
		}
	}
	item, err := s.service.UpdateProject(ctx, service.UpdateProjectInput{
		ID:    req.Msg.Id,
		Name:  req.Msg.Name,
		Key:   req.Msg.Key,
		Color: req.Msg.Color,
	})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.UpdateProjectResponse{Project: toProtoProject(item)}), nil
}

func (s *ProjectService) DeleteProject(ctx context.Context, req *connect.Request[appv1.DeleteProjectRequest]) (*connect.Response[appv1.DeleteProjectResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if err := s.service.DeleteProject(ctx, req.Msg.Id); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.DeleteProjectResponse{}), nil
}

func (s *ProjectService) ArchiveProject(ctx context.Context, req *connect.Request[appv1.ArchiveProjectRequest]) (*connect.Response[appv1.ArchiveProjectResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	item, err := s.service.ArchiveProject(ctx, req.Msg.Id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.ArchiveProjectResponse{Project: toProtoProject(item)}), nil
}

func toProtoProject(item service.Project) *appv1.Project {
	return &appv1.Project{
		Id:       item.ID,
		Name:     item.Name,
		Key:      item.Key,
		Color:    item.Color,
		Archived: item.Archived,
		InUse:    item.InUse,
	}
}
