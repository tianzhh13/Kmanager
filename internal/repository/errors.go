package repository

import "errors"

// 定义常见的仓库错误
var (
	ErrUserNotFound    = errors.New("user not found")
	ErrClusterNotFound = errors.New("cluster not found")
	ErrTopicNotFound   = errors.New("topic not found")
	ErrACLNotFound     = errors.New("acl not found")
)
