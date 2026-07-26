#pragma once

#include <map>
#include <string>

namespace detectors {

// CS2 competitive/MM tick rate. Demos from other rates rescale the per-tick
// thresholds, but 64 is the case the defaults are tuned against.
constexpr double kTickRate = 64.0;

// One tick of observable player state -- the subset of a parsed .dem frame the
// detectors actually need. A real parser (demoinfocs-golang, or a Rust/C++
// binding over the CS2 protobufs) populates a vector<Tick> per player.
struct Tick {
    int tick = 0;
    double pitch = 0.0;         // degrees, -89 (up) .. +89 (down)
    double yaw = 0.0;           // degrees, wraps at +/-180
    bool on_ground = true;
    bool jump_pressed = false;
    bool firing = false;
};

// The result of one detector over one player's trace.
struct Verdict {
    std::string name;
    double score = 0.0;         // 0.0 clean .. 1.0 damning
    std::string detail;
    std::map<std::string, double> evidence;

    // Same review threshold the Python reference used. Deliberately high: at
    // CS2's population a false positive is far more expensive than a miss, so
    // a "flag" is a hand-review trigger, not a conviction.
    bool flagged() const { return score >= 0.7; }
};

}  // namespace detectors
