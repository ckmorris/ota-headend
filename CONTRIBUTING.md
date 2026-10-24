# Contributing to OTA Headend

Chris Morris maintains the official project and approves changes to it. Read
[LICENSE.md](LICENSE.md) and [CLA.md](CLA.md) before submitting code. The project
uses PolyForm Noncommercial 1.0.0, a source-available license that permits
noncommercial use, modification, forks, and redistribution under its terms.

Report bugs or propose changes through the official project repository on the
[ckmorris GitHub account](https://github.com/ckmorris). Explain the problem,
expected behavior, and a way to reproduce it. For vulnerabilities, use
[SECURITY.md](SECURITY.md).

To contribute code:

1. Fork the repository and create a branch for your proposed change. Preserve
   the license and required notices in copies you share.
2. Keep the change focused. Add meaningful tests for ownership, lifecycle,
   command behavior, or transport behavior where relevant.
3. Run `make check`. Tests use fake hardware/processes and must never claim an
   actual tuner automatically.
4. Open a pull request in the official repository explaining the change and how
   you verified it.
5. Include the acceptance statement in CLA.md with your name, date, and any
   employer authorization, and identify any third-party material and licenses.

Chris Morris decides whether to merge and release your contribution. The CLA
permits licensing submitted work under other terms, including paid commercial
licenses. Contributors retain their own copyright; submission does not create
shared project control or a right to payment. Rights to use, modify, and share
the project remain governed by LICENSE.md unless separately agreed in writing.
Do not send credentials, saved tuner claim keys, private configuration,
copyrighted broadcast samples, or material you cannot authorize us to use.
