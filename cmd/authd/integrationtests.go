//go:build integrationtests

package main

import (
	"os"
	"strings"

	permissionstestutils "github.com/ubuntu/authd/internal/services/permissions/testutils"
	localgroupstestutils "github.com/ubuntu/authd/internal/users/localgroups/testutils"
)

// load any behaviour modifiers from env variable.
func init() {
	if os.Getenv("AUTHD_INTEGRATIONTESTS_CURRENT_USER_AS_ROOT") != "" {
		permissionstestutils.DefaultCurrentUserAsRoot()
	}

	gpasswdArgs := os.Getenv("TESTS_GPASSWD_ARGS")
	grpFilePath := os.Getenv("TESTS_GPASSWD_GRP_FILE_PATH")
	if gpasswdArgs == "" || grpFilePath == "" {
		panic("TESTS_GPASSWD_ARGS and TESTS_GPASSWD_GRP_FILE_PATH must be set")
	}
	localgroupstestutils.SetGpasswdCmd(strings.Split(gpasswdArgs, "-sep-"))
	localgroupstestutils.SetGroupPath(grpFilePath)
}
