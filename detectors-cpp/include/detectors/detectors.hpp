#pragma once

#include <string>
#include <utility>
#include <vector>

#include "detectors/tick.hpp"

namespace detectors {

// Shortest signed angular distance, so 179 -> -179 reads as +2, not -358.
double wrap180(double degrees);

// Per-tick signed yaw change across a trace (size N-1 for N ticks).
std::vector<double> yaw_deltas(const std::vector<Tick>& ticks);

// Each detector tests one physical/statistical invariant that a human hand
// satisfies and injected code does not. See detectors.cpp for the reasoning
// behind each one.
Verdict detect_antiaim(const std::vector<Tick>& ticks, int window = 128);
Verdict detect_aim_snap(const std::vector<Tick>& ticks, int pre = 8, int post = 3);
Verdict detect_no_recoil(const std::vector<Tick>& ticks);
Verdict detect_bhop(const std::vector<Tick>& ticks);

// Spread removal needs impact positions rather than the tick stream, so it
// runs off bullet_impact events: (x, y) offsets from the aim point in engine
// units, plus shot distance.
Verdict detect_no_spread(const std::string& weapon,
                         const std::vector<std::pair<double, double>>& impacts,
                         double distance);

// The four tick-stream detectors, in display order.
std::vector<Verdict> review(const std::vector<Tick>& ticks);

}  // namespace detectors
