# CS2 behavioural cheat detectors (C++)

Offline, server-side **detection** of cheat behaviour in CS2 demos. This is the
anti-cheat side of the problem: given a parsed `.dem`, flag players whose
recorded motion violates a physical or statistical invariant that a human hand
satisfies and injected code does not. It reads recorded matches only — no memory
access, no live client, nothing that touches a running game.

This is a C++ port of a Python reference; the two produce the same verdicts.

## What it looks for

| Detector      | Invariant a human satisfies                              | How the cheat breaks it                              |
|---------------|----------------------------------------------------------|------------------------------------------------------|
| `anti-aim`    | Yaw over a short window is *concentrated*                 | Desync makes recorded yaw uniform or bimodal         |
| `aim-snap`    | Flicks have *mass* — a 5–15 tick acceleration ramp        | Aimbot writes the angle in one tick; silent aim snaps out and back |
| `no-recoil`   | Sustained fire forces the aim path to *move*              | Removing recoil leaves a flat, sub-0.05°/tick spray  |
| `bhop-script` | Scroll-spam hop timing is *probabilistic* and decays      | A script lands every hop with no variance            |
| `no-spread`   | Impacts scatter across the weapon's inaccuracy cone       | Spread removal collapses the cloud below the cone     |

Each detector returns a `Verdict` with a 0–1 score (`flagged()` at ≥ 0.70) and a
human-readable reason. The threshold is deliberately high: at CS2's population a
false positive costs far more than a miss, so a flag is a hand-review trigger,
not a conviction.

## Layout

```
include/detectors/   tick.hpp (Tick, Verdict), detectors.hpp (the API)
src/detectors.cpp    the five invariant checks + review()
src/synth.{hpp,cpp}  synthetic clean/cheating traces to exercise them
src/main.cpp         prints the verdict table; --check asserts the signatures
```

## Build & run

```sh
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build
./build/signatures            # print the verdict table
./build/signatures --check    # assert every signature fires correctly (CI)
ctest --test-dir build        # same, via CTest
```

## Wiring in real demos

`main.cpp` feeds synthetic traces to the detectors so the project runs with no
data on hand. For real analysis, replace that source with a `.dem` parser and
populate one `std::vector<Tick>` per player:

- **Go** — `demoinfocs-golang` is the reference implementation.
- **Rust** — the `cs-demo-parser` / `csgoproto` line reads CS2 demos natively.
- **C++** — bind directly against Valve's demo protobufs.

The thresholds here are tuned against synthetic data and are **not** validated.
Production use means fitting them on labelled demos (Overwatch-convicted accounts
vs. known-clean matches) and optimising for precision at CS2's scale.
