package dash

import (
	"context"

	"entgo.io/ent/dialect/sql"
	dashv1 "github.com/go-sphere/sphere-telegram-layout/api/dash/v1"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/conv"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent/admin"
	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/render/entbind"
	"github.com/go-sphere/sphere/utils/secure"
)

var _ dashv1.AdminServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateAdmin(ctx context.Context, request *dashv1.CreateAdminRequest) (*dashv1.CreateAdminResponse, error) {
	request.Admin.Avatar = s.storage.ExtractKeyFromURL(request.Admin.Avatar)
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

func (s *Service) DeleteAdmin(ctx context.Context, request *dashv1.DeleteAdminRequest) (*dashv1.DeleteAdminResponse, error) {
	value, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	if value == request.Id {
		return nil, dashv1.AdminError_ADMIN_ERROR_CANNOT_DELETE_SELF
	}
	err = s.db.Admin.DeleteOneID(request.Id).Exec(ctx)
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

func (s *Service) UpdateAdmin(ctx context.Context, req *dashv1.UpdateAdminRequest) (*dashv1.UpdateAdminResponse, error) {
	if req.Admin.Password != "" {
		hashed, err := secure.CryptPassword(req.Admin.Password)
		if err != nil {
			return nil, err
		}
		req.Admin.Password = hashed
	}
	u, err := entbind.UpdateOneAdmin(
		s.db.Admin.UpdateOneID(req.Admin.Id),
		req.Admin,
		entbind.IgnoreSetZeroField(admin.FieldPassword),
	).Save(ctx)
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
