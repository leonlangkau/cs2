#pragma once

#include <random>
#include <vector>

#include "detectors/tick.hpp"

// Synthetic traces to exercise the detectors. Real validation needs labelled
// demos -- Overwatch-convicted accounts on one end, a pro match on the other.
// These exist so the code runs and so you can see the shape of each signature
// next to a clean baseline. Exact scores differ from the Python reference (a
// different RNG), but each trace lands on the same verdict.
namespace synth {

using detectors::Tick;
using RNG = std::mt19937;

std::vector<Tick> clean_player(RNG& rng, int n = 640);
std::vector<Tick> spinbot(RNG& rng, int n = 640);
std::vector<Tick> silent_aim(RNG& rng, int n = 640);
std::vector<Tick> no_recoil(RNG& rng, int n = 640);
std::vector<Tick> bhop_human(RNG& rng, int n = 640);
std::vector<Tick> bhop_script(RNG& rng, int n = 640);

}  // namespace synth
