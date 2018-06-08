/*
 * Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
 *
 * This program and the accompanying materials are made available under
 * the terms of the under the Apache License, Version 2.0 (the "License”);
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
 package cli

import (
	"fmt"
	"strings"
)

type DiagnosticFunc func(message string, command string)

type ArgConsumer struct {
	positionalArgs []string
	command        string
	consumed       map[int]struct{}
	diagnose       DiagnosticFunc
}

func NewArgConsumer(positionalArgs []string, diagnose DiagnosticFunc) *ArgConsumer {
	consumed := make(map[int]struct{})
	consumed[0] = struct{}{}
	return &ArgConsumer{
		positionalArgs: positionalArgs,
		command:        positionalArgs[0],
		consumed:       consumed,
		diagnose:       diagnose,
	}
}

func (ac *ArgConsumer) Consume(arg int, argDescription string) string {
	if len(ac.positionalArgs) < arg+1 || ac.positionalArgs[arg] == "" {
		ac.diagnose(fmt.Sprintf("Incorrect usage: %s not specified.", argDescription), ac.command)
		return ""
	}
	ac.consumed[arg] = struct{}{}
	return ac.positionalArgs[arg]
}

func (ac *ArgConsumer) ConsumeOptional(arg int, argDescription string) string {
	if len(ac.positionalArgs) < arg+1 || ac.positionalArgs[arg] == "" {
		return ""
	}
	ac.consumed[arg] = struct{}{}
	return ac.positionalArgs[arg]
}

func (ac *ArgConsumer) CheckAllConsumed() {
	if len(ac.consumed) < len(ac.positionalArgs) {
		extra := []string{}
		for i, arg := range ac.positionalArgs {
			if _, consumed := ac.consumed[i]; !consumed {
				extra = append(extra, arg)
			}
		}
		argumentInsert := "argument"
		if len(extra) > 1 {
			argumentInsert = "arguments"
		}
		ac.diagnose(fmt.Sprintf("Incorrect usage: invalid %s '%s'.", argumentInsert, strings.Join(extra, " ")), ac.command)
	}
}
