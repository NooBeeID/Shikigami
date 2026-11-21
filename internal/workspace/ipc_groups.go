package workspace

import (
	"context"
	"encoding/json"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

type groupsEvent struct {
	Kind string `json:"kind"`
	GroupsState
}

type createGroupResult struct {
	GroupsState
	GroupID string `json:"groupId"`
}

func publishGroups(srv *ipc.Server, kind string, st GroupsState) {
	srv.Publish("workspace.groupsChanged", groupsEvent{Kind: kind, GroupsState: st})
}

func registerGroupMethods(srv *ipc.Server, s *Store) {
	srv.Register("workspace.group.create", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			Name        string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		st, id, err := s.CreateGroup(p.WorkspaceID, p.Name)
		if err != nil {
			return nil, mapErr(err)
		}
		publishGroups(srv, "groupCreated", st)
		return createGroupResult{GroupsState: st, GroupID: id}, nil
	})

	srv.Register("workspace.group.rename", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			GroupID     string `json:"groupId"`
			Name        string `json:"name"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		st, changed, err := s.RenameGroup(p.WorkspaceID, p.GroupID, p.Name)
		if err != nil {
			return nil, mapErr(err)
		}
		if changed {
			publishGroups(srv, "groupRenamed", st)
		}
		return st, nil
	})

	srv.Register("workspace.group.delete", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			GroupID     string `json:"groupId"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		st, err := s.DeleteGroup(p.WorkspaceID, p.GroupID)
		if err != nil {
			return nil, mapErr(err)
		}
		publishGroups(srv, "groupDeleted", st)
		return st, nil
	})

	srv.Register("workspace.group.reorder", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			GroupID     string `json:"groupId"`
			ToIndex     int    `json:"toIndex"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		st, changed, err := s.ReorderGroup(p.WorkspaceID, p.GroupID, p.ToIndex)
		if err != nil {
			return nil, mapErr(err)
		}
		if changed {
			publishGroups(srv, "groupReordered", st)
		}
		return st, nil
	})

	srv.Register("workspace.group.setActive", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			GroupID     string `json:"groupId"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		st, changed, err := s.SetActiveGroup(p.WorkspaceID, p.GroupID)
		if err != nil {
			return nil, mapErr(err)
		}
		if changed {
			publishGroups(srv, "groupActivated", st)
		}
		return st, nil
	})

	srv.Register("workspace.group.moveSession", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct {
			WorkspaceID string `json:"workspaceId"`
			SessionID   string `json:"sessionId"`
			ToGroupID   string `json:"toGroupId"`
		}
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		st, changed, err := s.MoveSession(p.WorkspaceID, p.SessionID, p.ToGroupID)
		if err != nil {
			return nil, mapErr(err)
		}
		if changed {
			publishGroups(srv, "sessionMoved", st)
		}
		return st, nil
	})
}
