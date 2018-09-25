// +build linux

/*
   Copyright The containerd Authors.

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

package fs

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/containerd/continuity/testutil"
)

func testSupportsDType(t *testing.T, expected bool, mkfs ...string) {
	testutil.RequiresRoot(t)
	mnt, err := ioutil.TempDir("", "containerd-fs-test-supports-dtype")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(mnt)

	deviceName, cleanupDevice, err := testutil.NewLoopback(100 << 20) // 100 MB
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mkfs[0], append(mkfs[1:], deviceName)...).CombinedOutput(); err != nil {
		// not fatal
		t.Skipf("could not mkfs (%v) %s: %v (out: %q)", mkfs, deviceName, err, string(out))
	}
	if out, err := exec.Command("mount", deviceName, mnt).CombinedOutput(); err != nil {
		// not fatal
		t.Skipf("could not mount %s: %v (out: %q)", deviceName, err, string(out))
	}
	defer func() {
		testutil.Unmount(t, mnt)
		cleanupDevice()
	}()
	// check whether it supports d_type
	result, err := SupportsDType(mnt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Supports d_type: %v", result)
	if expected != result {
		t.Fatalf("expected %+v, got: %+v", expected, result)
	}
}

func TestSupportsDTypeWithFType0XFS(t *testing.T) {
	testSupportsDType(t, false, "mkfs.xfs", "-m", "crc=0", "-n", "ftype=0")
}

func TestSupportsDTypeWithFType1XFS(t *testing.T) {
	testSupportsDType(t, true, "mkfs.xfs", "-m", "crc=0", "-n", "ftype=1")
}

func TestSupportsDTypeWithExt4(t *testing.T) {
	testSupportsDType(t, true, "mkfs.ext4", "-F")
}
