/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
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
 *
 * Copyright 2020 Red Hat, Inc.
 *
 */

package tests

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	expect "github.com/google/goexpect"
	"google.golang.org/grpc/codes"
	"k8s.io/utils/net"

	v1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"
	"kubevirt.io/kubevirt/tests/libvmi"
)

var (
	shellSuccess = regexp.MustCompile(libvmi.RetValue("0"))
	shellFail    = regexp.MustCompile(libvmi.RetValue("[1-9].*"))
)

// PingFromVMConsole performs a ping through the provided VMI console.
// Optional arguments for the ping command may be provided, overwirting the default ones.
// (default ping options: "-c 1, -w 5")
// Note: The maximum overall command timeout is 10 seconds.
func PingFromVMConsole(vmi *v1.VirtualMachineInstance, ipAddr string, args ...string) error {
	const maxCommandTimeout = 10 * time.Second

	pingString := "ping"
	if net.IsIPv6String(ipAddr) {
		pingString = "ping -6"
	}

	if len(args) == 0 {
		args = []string{"-c 1", "-w 5"}
	}
	args = append([]string{pingString, ipAddr}, args...)
	cmdCheck := strings.Join(args, " ") + "\n"

	err := vmiConsoleExpectBatch(vmi, []expect.Batcher{
		&expect.BSnd{S: "\n"},
		&expect.BExp{R: libvmi.PromptExpression},
		&expect.BSnd{S: cmdCheck},
		&expect.BExp{R: libvmi.PromptExpression},
		&expect.BSnd{S: "echo $?\n"},
		&expect.BCas{C: []expect.Caser{
			&expect.Case{
				R: shellSuccess,
				T: expect.OK(),
			},
			&expect.Case{
				R: shellFail,
				T: expect.Fail(expect.NewStatus(codes.Unavailable, "ping failed")),
			},
		}},
	}, maxCommandTimeout)
	if err != nil {
		return fmt.Errorf("Failed to ping VMI %s, error: %v", vmi.Name, err)
	}
	return nil
}

func vmiConsoleExpectBatch(vmi *v1.VirtualMachineInstance, expected []expect.Batcher, timeout time.Duration) error {
	virtClient, err := kubecli.GetKubevirtClient()
	if err != nil {
		return err
	}

	expecter, _, err := libvmi.NewConsoleExpecter(virtClient, vmi, 30*time.Second)
	if err != nil {
		return err
	}
	defer expecter.Close()

	resp, err := expecter.ExpectBatch(expected, timeout)
	if err != nil {
		log.DefaultLogger().Object(vmi).Infof("%v", resp)
	}
	return err
}
