/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functiontemplates

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"
)

type GithubFetcherTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *GithubFetcherTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *GithubFetcherTestSuite) TestFetch() {
	suite.T().Skip("TestGithubFetcher not supported - can't get NUCLIO_GITHUB_API_TOKEN")
	githuAPItoken := os.Getenv("NUCLIO_GITHUB_API_TOKEN")
	templateFetcher, err := NewGithubFunctionTemplateFetcher(suite.logger,
		"nuclio-templates",
		"nuclio",
		"master",
		githuAPItoken)
	suite.Require().NoError(err)

	templates, err := templateFetcher.Fetch()
	suite.Require().NoError(err)

	suite.logger.DebugWith("Fetcher ended", "templates", templates)
}

func TestGithubFetcher(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(GithubFetcherTestSuite))
}