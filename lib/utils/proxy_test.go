/*
Copyright 2017 Gravitational, Inc.

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
package utils

import (
	"fmt"

	"gopkg.in/check.v1"
)

type ProxySuite struct{}

var _ = check.Suite(&ProxySuite{})
var _ = fmt.Printf

func (s *ProxySuite) SetUpSuite(c *check.C) {
	InitLoggerForTests()
}
func (s *ProxySuite) TearDownSuite(c *check.C) {}
func (s *ProxySuite) SetUpTest(c *check.C)     {}
func (s *ProxySuite) TearDownTest(c *check.C)  {}

func (s *ProxySuite) TestProxyDial(c *check.C) {
}
