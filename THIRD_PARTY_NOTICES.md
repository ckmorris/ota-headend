# Third-party notices

OTA Headend is Copyright © 2026 Chris Morris. Its
[PolyForm Noncommercial 1.0.0 license](LICENSE.md) applies to the project's
original code and documentation, not to rights granted
independently by third-party licenses.

## Go runtime and standard library

The source uses Go's runtime and standard library and has no third-party Go
module dependencies. Built releases include Go code under its BSD-style license.
Each binary release archive includes the exact build toolchain's license in
`GO-LICENSE.txt`, its patent notice when present in `GO-PATENTS.txt`, and
toolchain-vendored license/patent/notice files in `GO-VENDOR-NOTICES/`. These are
third-party notices; OTA Headend's own code remains subject to LICENSE.md.

The Go license text is reproduced here for source users:

```text
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## Separately installed executables

- [TSDuck](https://tsduck.io/), including `tsp`, is an independent project with
  its own license and optional-component notices. See the documentation and
  license supplied with the exact TSDuck package you install.
- [SiliconDust libhdhomerun / hdhomerun_config](https://github.com/Silicondust/libhdhomerun)
  is an independent project, published under LGPL-2.1 terms. See its upstream
  license and the notices supplied with your distribution's package.

These executables are invoked as separate processes and are not bundled into
OTA Headend's release archives. You must install them separately. HDHomeRun and
SiliconDust are names of their respective owners; no affiliation or endorsement
is claimed. Broadcasts and other received media are not part of this software
distribution.
