package repository

import "errors"

var (
	ErrBadRequest         = errors.New("repository: bad request")
	ErrConflict           = errors.New("repository: conflict")
	ErrForbidden          = errors.New("repository: forbidden")
	ErrNotFound           = errors.New("repository: not found")
	ErrUsernameDuplicated = errors.New("repository: username duplicated")
	ErrRoleNameDuplicated = errors.New("repository: role name duplicated")
	ErrRoleSIDDuplicated  = errors.New("repository: role sid duplicated")
)
