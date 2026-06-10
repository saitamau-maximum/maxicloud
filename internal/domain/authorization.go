package domain

import (
	"context"
	"fmt"
	"slices"
	"time"
)

type Role int

const (
	RoleEditor Role = iota + 1
	RoleAdmin
	RoleOwner
)

func ParseRole(s string) (Role, error) {
	switch s {
	case "editor":
		return RoleEditor, nil
	case "admin":
		return RoleAdmin, nil
	default:
		return 0, fmt.Errorf("unknown role: %s", s)
	}
}

func (r Role) String() string {
	switch r {
	case RoleEditor:
		return "editor"
	case RoleAdmin:
		return "admin"
	case RoleOwner:
		return "owner"
	default:
		return "unknown"
	}
}

// Maximum の Admin Role を持つユーザーは全てのリソースに対して全ての操作を行える
const GlobalAdminRole = "admin"

type Permission int

const (
	// Application
	PermissionWriteApplication Permission = iota
	PermissionDeleteApplication

	// Deploy
	PermissionTriggerDeploy

	// Project
	PermissionWriteProject // Project の作成は全ての Member が可能でこの権限は必要ない
	PermissionDeleteProject
	PermissionManageMembers
)

// rolePermissions は各 role が持つ permission の束（独立した権限マトリクス）。
// 束同士の上下関係は前提しないので、非単調な権限にしたければ各行をいじるだけでよい。
var rolePermissions = map[Role][]Permission{
	RoleEditor: {
		PermissionWriteApplication,
		PermissionTriggerDeploy,
		PermissionWriteProject,
	},
	RoleAdmin: {
		PermissionWriteApplication,
		PermissionTriggerDeploy,
		PermissionWriteProject,
		PermissionDeleteApplication,
		PermissionManageMembers,
	},
	RoleOwner: {
		PermissionWriteApplication,
		PermissionTriggerDeploy,
		PermissionWriteProject,
		PermissionDeleteApplication,
		PermissionManageMembers,
		PermissionDeleteProject,
	},
}

// RoleHasPermission は role の束に perm が含まれるかを返す。
func RoleHasPermission(role Role, perm Permission) bool {
	return slices.Contains(rolePermissions[role], perm)
}

// Principal は認可判定の主体を表す。OIDC 由来かどうかは domain は関知しない。
type Principal struct {
	ID    string
	Roles []string // OIDC グループ等の外部 role 名
}

// Can は照合済みの付与 role 集合と project のオーナーシップを元に perm を持つかを返す。
func (p Principal) Can(perm Permission, project *Project, grantedRoles []Role) bool {
	if slices.Contains(p.Roles, GlobalAdminRole) {
		return true
	}
	if project.OwnerID == p.ID && RoleHasPermission(RoleOwner, perm) {
		return true
	}
	for _, r := range grantedRoles {
		if RoleHasPermission(r, perm) {
			return true
		}
	}
	return false
}

// ProjectMember はユーザー単位のプロジェクトロール付与を表す。
type ProjectMember struct {
	ID        string
	ProjectID string
	UserID    string
	Role      Role
	CreatedAt time.Time
	CreatedBy string
}

type ProjectMemberRepository interface {
	GetByUser(ctx context.Context, projectID, userID string) (*ProjectMember, error)
	ListByProject(ctx context.Context, projectID string) ([]ProjectMember, error)
	Add(ctx context.Context, member ProjectMember) error
	Remove(ctx context.Context, id string) error
	UpdateRole(ctx context.Context, id string, role Role) error
}

// ProjectGroupRole は OIDC グループ単位のプロジェクトロール付与を表す。
type ProjectGroupRole struct {
	ID        string
	ProjectID string
	OIDCRole  string
	Role      Role
	CreatedAt time.Time
	CreatedBy string
}

type ProjectGroupRoleRepository interface {
	ListByGroups(ctx context.Context, projectID string, oidcRoles []string) ([]ProjectGroupRole, error)
	ListByProject(ctx context.Context, projectID string) ([]ProjectGroupRole, error)
	Add(ctx context.Context, g ProjectGroupRole) error
	Remove(ctx context.Context, id string) error
	UpdateRole(ctx context.Context, id string, role Role) error
}
