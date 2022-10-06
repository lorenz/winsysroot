# WinSysRoot

_Automatically assemble Windows Sysroots directly from Microsoft sources_

## Features

- Very small, minimal sysroots. The x64 tarball zstd-compressed is just 106MiB.
- Fully cross-platform, no proprietary code involved in the toolchain. Works with any
  properly-configured LLVM.
- Very fast. The compressed x64 tarball takes ~20s to download and assemble.
- Selectable Windows SDK version
- Integrated VFS overlay to make it work on case-sensitive filesystems.
- Near 100% compatibility with normal Microsoft MSVC. No Cygwin/MinGW/MSYS2.

## Installation

This requires an up-to-date Go toolchain, currently there are no precompiled binaries provided.

```sh
go install git.dolansoft.org/lorenz/winsysroot@latest
```

It also requires LLVM 15 or higher with lld-link, which you need to install for your platform.

## Usage

First, generate a sysroot. If you're content with the defaults (x64 only with SDK 10.0.20348), just
call

```
winsysroot --out-dir=somewere/my-sysroot
```

The full option list can be shown using `--help`.

Note that this does NOT need a case-insensitive directory on Linux/MacOS. It doesn't break it, but
it is also not required.

This sysroot can then be used either standalone or with the included wrapper scripts:

```sh
WINSYSROOT=somewere/my-sysroot wrappers/clang-cl-x64 /o examples/helloworld-x64.exe examples/helloworld.cc
```

If your clang-cl is not called `clang-cl`, you can set the `CLANG_CL` environment variable to what
it is in your environment.

## Notes

- arm64ec is VERY new and as of LLVM 15 does not fully work.
- The tarball has a hardcoded VFS location at /winsysroot which you need to change to the real
  unpacked path as the driver discovery does not go through the VFS overlay yet and thus passing
  /winsysroot as path doesn't work.

## Is this legal?

Probably. I'm not a lawyer and this is not legal advice, but I'm not distributing any
non-redistributable Microsoft content or circumventing any licensing schemes, you're downloading all
content directly from Microsoft's servers (just a lot more efficiently). Note that you cannot
legally distribute any sysroots generated from this tool without Microsoft's permission.
