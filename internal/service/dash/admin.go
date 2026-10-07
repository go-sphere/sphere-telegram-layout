package dash

import (
	"context"
	"strings"

	"entgo.io/ent/dialect/sql"
	dashv1 "github.com/go-sphere/sphere-telegram-layout/api/dash/v1"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/conv"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/dao"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/admin"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/adminsession"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/render/entbind"
	"github.com/go-sphere/sphere/utils/secure"
)

var _ dashv1.AdminServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateAdmin(ctx context.Context, request *dashv1.CreateAdminRequest) (*dashv1.CreateAdminResponse, error) {
	request.Admin.Avatar = s.storage.ExtractKeyFromURL(request.Admin.Avatar)
	request.Admin.Username = NormalizeUsername(request.Admin.Username)
	hashed, err := secure.CryptPassword(request.Admin.Password)
	if err != nil {
		return nil, err
	}
	request.Admin.Password = hashed
	u, err := entbind.CreateAdmin(s.db.Admin.Create(), request.Admin, entbind.IgnoreField(admin.FieldID)).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.CreateAdminResponse{
		Admin: s.render.Admin(u),
	}, nil
}

// DeleteAdmin deletes another admin and revokes their refresh sessions.
// Already issued access tokens stay valid until they expire.
func (s *Service) DeleteAdmin(ctx context.Context, request *dashv1.DeleteAdminRequest) (*dashv1.DeleteAdminResponse, error) {
	value, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	if value == request.Id {
		return nil, dashv1.AdminError_ADMIN_ERROR_CANNOT_DELETE_SELF
	}
	err = dao.WithTxEx(ctx, s.db.Client, func(ctx context.Context, client *ent.Client) error {
		if err := client.Admin.DeleteOneID(request.Id).Exec(ctx); err != nil {
			return err
		}
		return revokeAdminSessions(ctx, client, request.Id)
	})
	if err != nil {
		return nil, err
	}
	return &dashv1.DeleteAdminResponse{}, nil
}

func (s *Service) GetAdmin(ctx context.Context, request *dashv1.GetAdminRequest) (*dashv1.GetAdminResponse, error) {
	adm, err := s.db.Admin.Get(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &dashv1.GetAdminResponse{
		Admin: s.render.Admin(adm),
	}, nil
}

func (s *Service) ListAdmins(ctx context.Context, request *dashv1.ListAdminsRequest) (*dashv1.ListAdminsResponse, error) {
	query := s.db.Admin.Query()
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	totalPage, pageSize := conv.Page(count, int(request.PageSize))
	all, err := query.Clone().Limit(pageSize).Order(admin.ByID(sql.OrderDesc())).Offset(pageSize * int(request.Page)).All(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.ListAdminsResponse{
		Admins:    conv.Map(all, s.render.Admin),
		TotalSize: int64(count),
		TotalPage: int64(totalPage),
	}, nil
}

// UpdateAdmin updates an admin. Setting a new password also revokes all of
// that admin's refresh sessions, so a leaked refresh token stops working.
func (s *Service) UpdateAdmin(ctx context.Context, req *dashv1.UpdateAdminRequest) (*dashv1.UpdateAdminResponse, error) {
	req.Admin.Username = NormalizeUsername(req.Admin.Username)
	passwordChanged := req.Admin.Password != ""
	if passwordChanged {
		hashed, err := secure.CryptPassword(req.Admin.Password)
		if err != nil {
			return nil, err
		}
		req.Admin.Password = hashed
	}
	u, err := dao.WithTx(ctx, s.db.Client, func(ctx context.Context, client *ent.Client) (*ent.Admin, error) {
		u, err := entbind.UpdateOneAdmin(
			client.Admin.UpdateOneID(req.Admin.Id),
			req.Admin,
			entbind.IgnoreSetZeroField(admin.FieldPassword),
		).Save(ctx)
		if err != nil {
			return nil, err
		}
		if passwordChanged {
			if err := revokeAdminSessions(ctx, client, u.ID); err != nil {
				return nil, err
			}
		}
		return u, nil
	})
	if err != nil {
		return nil, err
	}
	return &dashv1.UpdateAdminResponse{
		Admin: s.render.Admin(u),
	}, nil
}

func (s *Service) ListAdminRoles(ctx context.Context, request *dashv1.ListAdminRolesRequest) (*dashv1.ListAdminRolesResponse, error) {
	return &dashv1.ListAdminRolesResponse{
		Roles: []string{
			PermissionAll,
			PermissionAdmin,
		},
	}, nil
}

// NormalizeUsername is the canonical form of an admin username: trimmed and
// lowercased. Create, update, seed and login all use it, so the case-sensitive
// unique index and the exact-match login lookup agree.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// revokeAdminSessions revokes every refresh session of the admin uid.
func revokeAdminSessions(ctx context.Context, client *ent.Client, uid int64) error {
	return client.AdminSession.Update().
		Where(adminsession.UIDEQ(uid), adminsession.IsRevoked(false)).
		SetIsRevoked(true).
		Exec(ctx)
}
