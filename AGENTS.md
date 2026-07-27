# AGENTS.md

## Cursor Cloud specific instructions

### Repository layout
- The `main` branch currently contains only `README.md` (marketing/install notes). There is
  no build system, package manifest, or runnable application on `main` itself.
- The only actual source code is a self-contained C++17 CMake project under `detectors-cpp/`
  (an offline, read-only CS2 demo *cheat-detection* analysis library + `signatures` CLI).
  As of this writing it lives on the feature branch `claude/game-cheat-features-menu-9i6ik4`;
  build/run instructions below apply wherever that directory is present.

### Toolchain (already in the base image)
- `cmake` (3.28), `g++` (13.3), `make`, `clang` (18) are pre-installed. There are **no**
  third-party/library dependencies — the C++ project uses the standard library only, so there
  is nothing to `pip`/`npm`/`vcpkg` install.

### IMPORTANT compiler caveat (non-obvious)
- The `c++`/`cc` alternatives were pinned to **clang**, but clang has **no usable C++ standard
  library** in this image (no libc++, and it fails to find libstdc++: `cannot find -lstdc++` /
  `'vector' file not found`). **Use g++**, which works perfectly.
- The startup/update script repoints the `c++` alternative to `g++`
  (`sudo update-alternatives --set c++ /usr/bin/g++`), so a plain `cmake` build works.
- If you ever hit the clang error again (e.g. a fresh VM before the update script ran), either
  run that `update-alternatives` command, or configure with an explicit compiler:
  `cmake -S detectors-cpp -B detectors-cpp/build -DCMAKE_BUILD_TYPE=Release -DCMAKE_CXX_COMPILER=g++`.

### Build / run / test (from inside `detectors-cpp/`)
- Build:  `cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build`
- Run:    `./build/signatures`          (prints the verdict table over synthetic traces)
- Check:  `./build/signatures --check`   (asserts every detector signature fires; exit 0 = pass)
- Tests:  `ctest --test-dir build`       (runs the same `--check` assertion via CTest)
- Lint:   there is no separate linter; the build uses `-Wall -Wextra` and is expected to be
  warning-free. Treat any new warning as a lint failure.
