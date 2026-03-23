package service

import "errors"

var ErrInvalidHeartbeatConfig = errors.New("heartbeat_timeout must be greater than or equal to heartbeat_sec")
var ErrInvalidClientID = errors.New("invalid client id: must be 1-64 alphanumeric characters, underscore or hyphen")
var ErrClientAlreadyExists = errors.New("client already exists")
var ErrClientNotFound = errors.New("client not found")
var ErrClientNotOnline = errors.New("client not online")
var ErrProxyRuleLimitExceeded = errors.New("proxy rule limit exceeded")
