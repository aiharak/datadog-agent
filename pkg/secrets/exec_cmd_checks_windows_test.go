// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2018 Datadog, Inc.

// +build secrets,windows

package secrets

import (
	"os"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserOnWindows(t *testing.T) {
	defer func() {
		secretBackendCommand = ""
		secretBackendArguments = []string{}
		secretBackendTimeout = 0
	}()

	inputPayload := "{\"version\": \"" + payloadVersion + "\" , \"secrets\": [\"sec1\", \"sec2\"]}"

	secretBackendCommand = "./test/user/user"
	resp, err = execCommand(inputPayload)
	require.Nil(t, err)
	assert.Equal(t, []byte("Username: datadog_secretuser"), resp)
	// check that we're not running test as 'datadog_secretuser', to be
	// sure we executed secretBackendCommand with another user
	user, err := user.Current()
	require.Nil(t, err)
	assert.NotEqual(t, "datadog_secretuser", user.Username)
}