/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package server

import "google.golang.org/grpc/credentials"

// peerCredAuthInfo carries the OS-level credentials of the calling process
// (UID/GID/PID), as resolved by SO_PEERCRED on the underlying Unix socket.
// It is attached to the gRPC peer context by the credentials shim.
type peerCredAuthInfo struct {
	UID uint32
	GID uint32
	PID int32
}

// AuthType implements credentials.AuthInfo.
func (peerCredAuthInfo) AuthType() string { return "ucred" }

// newPeerCredentials returns a TransportCredentials implementation that
// inspects the peer of an incoming Unix socket connection. Returns nil on
// platforms that don't support SO_PEERCRED — the caller should treat nil as
// "no auth wrapper, fall back to socket file permissions".
func newPeerCredentials() credentials.TransportCredentials {
	return platformPeerCredentials()
}
