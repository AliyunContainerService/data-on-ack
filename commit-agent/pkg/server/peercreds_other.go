/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

//go:build !linux

package server

import "google.golang.org/grpc/credentials"

// On non-Linux platforms SO_PEERCRED is unavailable. Return nil to fall back
// to socket-permission based access control.
func platformPeerCredentials() credentials.TransportCredentials { return nil }
