#include "detectors/detectors.hpp"

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <iomanip>
#include <sstream>
#include <unordered_map>

namespace detectors {
namespace {

constexpr double kPi = 3.14159265358979323846;

double deg2rad(double deg) { return deg * kPi / 180.0; }

// Positive modulo, matching Python's `%` on floats (result takes the sign of
// the divisor). std::fmod alone would take the sign of the dividend and break
// wrap180 for negative angles.
double pmod(double a, double m) {
    double r = std::fmod(a, m);
    if (r < 0.0) r += m;
    return r;
}

double mean(const std::vector<double>& xs) {
    if (xs.empty()) return 0.0;
    double s = 0.0;
    for (double x : xs) s += x;
    return s / static_cast<double>(xs.size());
}

// Population standard deviation, matching statistics.pstdev.
double pstdev(const std::vector<double>& xs) {
    if (xs.empty()) return 0.0;
    double m = mean(xs);
    double acc = 0.0;
    for (double x : xs) acc += (x - m) * (x - m);
    return std::sqrt(acc / static_cast<double>(xs.size()));
}

double round3(double x) { return std::round(x * 1000.0) / 1000.0; }
double round4(double x) { return std::round(x * 10000.0) / 10000.0; }

// Format a fraction as an integer percent, e.g. 0.72 -> "72%".
std::string pct(double frac) {
    std::ostringstream os;
    os << static_cast<long long>(std::llround(frac * 100.0)) << '%';
    return os.str();
}

std::string fixed(double x, int places) {
    std::ostringstream os;
    os << std::fixed << std::setprecision(places) << x;
    return os.str();
}

}  // namespace

double wrap180(double degrees) { return pmod(degrees + 180.0, 360.0) - 180.0; }

std::vector<double> yaw_deltas(const std::vector<Tick>& ticks) {
    std::vector<double> out;
    if (ticks.size() < 2) return out;
    out.reserve(ticks.size() - 1);
    for (std::size_t i = 1; i < ticks.size(); ++i)
        out.push_back(wrap180(ticks[i].yaw - ticks[i - 1].yaw));
    return out;
}

// --------------------------------------------------------------------------
// 1. Anti-aim
// --------------------------------------------------------------------------
// Invariant: a human's yaw over any short window is *concentrated* -- you look
// roughly where you are going and roughly at what you shoot. Anti-aim desyncs
// the networked yaw from the played yaw, so the recorded yaw stops being
// concentrated and turns either uniform (spinbot) or bimodal (static desync).
//
// Circular variance is the right tool because yaw is an angle: a naive stdev
// over [-179, 179] reports huge spread for someone standing still looking
// north, which is exactly the false positive that sinks homegrown checks.
Verdict detect_antiaim(const std::vector<Tick>& ticks, int window) {
    Verdict worst{"anti-aim", 0.0, "no desync signature", {}};
    const int n = static_cast<int>(ticks.size());
    const int step = std::max(1, window / 2);
    const int limit = std::max(1, n - window + 1);

    for (int start = 0; start < limit; start += step) {
        const int end = std::min(start + window, n);
        const int len = end - start;
        if (len < window / 2) continue;

        double cos_sum = 0.0, sin_sum = 0.0;
        for (int i = start; i < end; ++i) {
            const double r = deg2rad(ticks[i].yaw);
            cos_sum += std::cos(r);
            sin_sum += std::sin(r);
        }
        const double resultant =
            std::hypot(cos_sum / len, sin_sum / len);
        const double circular_variance = 1.0 - resultant;

        int violent = 0, deltas = 0;
        for (int i = start + 1; i < end; ++i) {
            if (std::fabs(wrap180(ticks[i].yaw - ticks[i - 1].yaw)) > 90.0)
                ++violent;
            ++deltas;
        }
        const double violent_frac = deltas ? static_cast<double>(violent) / deltas : 0.0;

        // Either signal alone has an innocent cause -- a real spin when
        // checking your back, high variance on a rotation. Sustained together
        // they do not.
        const double score =
            std::min(1.0, circular_variance * 0.6 + violent_frac * 1.6);

        if (score > worst.score) {
            worst.score = score;
            worst.detail = "yaw uniformly distributed with " + pct(violent_frac) +
                           " of ticks exceeding 90 deg/tick";
            worst.evidence = {
                {"tick_range_start", static_cast<double>(ticks[start].tick)},
                {"tick_range_end", static_cast<double>(ticks[end - 1].tick)},
                {"circular_variance", round3(circular_variance)},
                {"violent_tick_fraction", round3(violent_frac)},
            };
        }
    }
    return worst;
}

// --------------------------------------------------------------------------
// 2. Aim snap (covers aimbot and silent aim)
// --------------------------------------------------------------------------
// Invariant: human aim has *mass*. A flick is a ramp -- accelerate, peak,
// decelerate, small overshoot correction -- over 5-15 ticks, no single tick
// carrying the whole movement. Code has no mass: an aimbot writes the target
// angle in one tick, a step function. Silent aim adds a second tell -- the
// angle is restored next tick, so delta[t] and delta[t+1] nearly cancel, a
// spike that leaves no trace in the aim path.
Verdict detect_aim_snap(const std::vector<Tick>& ticks, int pre, int post) {
    std::unordered_map<int, int> by_index;
    for (int i = 0; i < static_cast<int>(ticks.size()); ++i)
        by_index[ticks[i].tick] = i;

    std::vector<int> shots;
    for (int i = 0; i < static_cast<int>(ticks.size()); ++i)
        if (ticks[i].firing) shots.push_back(i);
    if (shots.empty())
        return Verdict{"aim-snap", 0.0, "no shots in trace", {}};

    int snaps = 0, silent = 0;
    double worst_ratio = 0.0;

    for (int shot_idx : shots) {
        const int i = by_index[ticks[shot_idx].tick];
        if (i - pre < 0 || i + post >= static_cast<int>(ticks.size())) continue;

        const std::vector<Tick> window(ticks.begin() + (i - pre),
                                       ticks.begin() + (i + post + 1));
        const std::vector<double> deltas = yaw_deltas(window);
        std::vector<double> mags;
        mags.reserve(deltas.size());
        for (double d : deltas) mags.push_back(std::fabs(d));

        const auto peak_it = std::max_element(mags.begin(), mags.end());
        const double peak = *peak_it;
        if (peak < 3.0) continue;  // tracking a target already under the crosshair

        double total = 0.0;
        for (double m : mags) total += m;
        const double rest = total - peak;
        // A ramped human flick spreads its travel -- peak ~25-35% of the
        // total. A step function puts nearly all of it in one tick.
        const double concentration = peak / (peak + rest + 1e-9);
        worst_ratio = std::max(worst_ratio, concentration);
        if (concentration > 0.8) ++snaps;

        const int peak_i = static_cast<int>(peak_it - mags.begin());
        if (peak_i + 1 < static_cast<int>(deltas.size())) {
            const double net = std::fabs(deltas[peak_i] + deltas[peak_i + 1]);
            if (peak > 15.0 && net < peak * 0.15) ++silent;
        }
    }

    const int judged = std::max<std::size_t>(1, shots.size());
    const double snap_rate = static_cast<double>(snaps) / judged;
    const double silent_rate = static_cast<double>(silent) / judged;
    const double score = std::min(1.0, snap_rate * 1.2 + silent_rate * 1.5);

    std::string detail;
    if (silent_rate > 0.1)
        detail = pct(silent_rate) +
                 " of shots snap to target and return within one tick (silent aim)";
    else if (snap_rate > 0.1)
        detail = pct(snap_rate) + " of shots preceded by single-tick step in yaw";
    else
        detail = "aim transitions show human acceleration profile";

    return Verdict{"aim-snap", score, detail,
                   {{"shots", static_cast<double>(shots.size())},
                    {"snap_rate", round3(snap_rate)},
                    {"silent_return_rate", round3(silent_rate)},
                    {"peak_concentration", round3(worst_ratio)}}};
}

// --------------------------------------------------------------------------
// 3. No recoil / no visual recoil
// --------------------------------------------------------------------------
// Invariant: sustained fire forces the aim path to *move*. Either the player
// compensates -- a noisy hand-driven pull -- or they do not and the view
// climbs. Both leave travel in the angles. Removing recoil removes the reason
// to compensate, so a long spray goes flat and, more tellingly, *smooth*: no
// human holds a rifle spray with sub-0.05 deg per-tick jitter.
namespace {
constexpr int kRifleSprayMinTicks = 12;
}  // namespace

Verdict detect_no_recoil(const std::vector<Tick>& ticks) {
    std::vector<std::pair<int, int>> sprays;  // [start, end) index ranges
    int cursor = 0;
    const int n = static_cast<int>(ticks.size());
    while (cursor < n) {
        if (ticks[cursor].firing) {
            const int start = cursor;
            while (cursor < n && ticks[cursor].firing) ++cursor;
            if (cursor - start >= kRifleSprayMinTicks)
                sprays.emplace_back(start, cursor);
        } else {
            ++cursor;
        }
    }

    if (sprays.empty())
        return Verdict{"no-recoil", 0.0, "no sustained sprays in trace", {}};

    double best_travel = 0.0, best_jitter = 0.0;
    int best_len = 0;
    bool have_best = false;

    for (const auto& [start, end] : sprays) {
        double travel = 0.0;
        std::vector<double> pitch_deltas;
        for (int i = start + 1; i < end; ++i) {
            const double d = ticks[i].pitch - ticks[i - 1].pitch;
            travel += std::fabs(d);
            pitch_deltas.push_back(d);
        }
        const double jitter = pstdev(pitch_deltas);
        const int len = end - start;
        // Keep the flattest spray -- the one hardest to explain honestly.
        if (!have_best || travel < best_travel) {
            best_travel = travel;
            best_jitter = jitter;
            best_len = len;
            have_best = true;
        }
    }

    const double expected = best_len * 0.25;  // deg of aggregate travel a human produces
    const double flatness = std::max(0.0, 1.0 - best_travel / expected);
    const double smoothness = std::max(0.0, 1.0 - best_jitter / 0.12);
    const double score = std::min(1.0, flatness * 0.6 + smoothness * 0.6);

    const std::string detail =
        std::to_string(best_len) + "-tick spray with " + fixed(best_travel, 2) +
        " deg total pitch travel (expected ~" + fixed(expected, 1) + ") and " +
        fixed(best_jitter, 3) + " deg jitter";

    return Verdict{"no-recoil", score, detail,
                   {{"sprays_examined", static_cast<double>(sprays.size())},
                    {"flattest_spray_ticks", static_cast<double>(best_len)},
                    {"pitch_travel_deg", round3(best_travel)},
                    {"per_tick_jitter_deg", round4(best_jitter)}}};
}

// --------------------------------------------------------------------------
// 4. Bunny hop scripting
// --------------------------------------------------------------------------
// Invariant: chaining hops means landing the jump input in the one-tick window
// where you have touched ground but not yet lost speed. Scroll-spam is
// *probabilistic* -- 30-60% of attempts, decaying over a long chain. A script
// hits it every time. The tell is not that the hops are good, it is that the
// success rate has no variance and the chain never decays.
Verdict detect_bhop(const std::vector<Tick>& ticks) {
    int landings = 0, perfect = 0, chain = 0, longest_chain = 0;

    for (std::size_t i = 1; i < ticks.size(); ++i) {
        const Tick& prev = ticks[i - 1];
        const Tick& cur = ticks[i];
        if (!prev.on_ground && cur.on_ground) {
            ++landings;
            if (cur.jump_pressed) {
                ++perfect;
                ++chain;
                longest_chain = std::max(longest_chain, chain);
            } else {
                chain = 0;
            }
        }
    }

    if (landings < 5)
        return Verdict{"bhop-script", 0.0, "too few landings to judge", {}};

    const double rate = static_cast<double>(perfect) / landings;
    // Weight both the rate and the longest unbroken chain -- a human gets lucky
    // for three hops, not thirty.
    const double score = std::min(
        1.0, std::max(0.0, (rate - 0.6) / 0.35) * 0.7 +
                 std::min(1.0, longest_chain / 15.0) * 0.5);

    const std::string detail =
        std::to_string(perfect) + "/" + std::to_string(landings) +
        " frame-perfect jump inputs, longest unbroken chain " +
        std::to_string(longest_chain);

    return Verdict{"bhop-script", score, detail,
                   {{"landings", static_cast<double>(landings)},
                    {"frame_perfect_rate", round3(rate)},
                    {"longest_chain", static_cast<double>(longest_chain)}}};
}

// --------------------------------------------------------------------------
// 5. Spread removal
// --------------------------------------------------------------------------
// Invariant: every shot's deviation is drawn from the weapon's inaccuracy cone,
// seeded per shot. Even standing still and tapping, impacts scatter. Removing
// spread collapses that scatter to the recoil pattern alone, so the impact
// cloud is tighter than the weapon can physically produce.
namespace {
// Standing, first-shot inaccuracy in engine units at 100 units of distance.
double weapon_cone(const std::string& weapon) {
    if (weapon == "ak47") return 1.85;
    if (weapon == "m4a1") return 1.55;
    if (weapon == "deagle") return 2.10;
    if (weapon == "awp") return 0.35;
    return -1.0;  // unknown
}
}  // namespace

Verdict detect_no_spread(const std::string& weapon,
                         const std::vector<std::pair<double, double>>& impacts,
                         double distance) {
    const double cone = weapon_cone(weapon);
    if (cone < 0.0 || impacts.size() < 8)
        return Verdict{"no-spread", 0.0, "insufficient impact data", {}};

    std::vector<double> radii;
    radii.reserve(impacts.size());
    for (const auto& [x, y] : impacts) radii.push_back(std::hypot(x, y));
    const double observed = mean(radii);
    const double expected = cone * (distance / 100.0) * 0.6;  // mean radius of a uniform cone

    const double ratio = observed / (expected + 1e-9);
    const double score = std::min(1.0, std::max(0.0, 1.0 - ratio / 0.45));

    const std::string detail =
        std::to_string(impacts.size()) + " " + weapon + " impacts average " +
        fixed(observed, 2) + "u from aim point; cone predicts ~" +
        fixed(expected, 2) + "u";

    return Verdict{"no-spread", score, detail,
                   {{"observed_mean_radius", round3(observed)},
                    {"expected_mean_radius", round3(expected)},
                    {"ratio", round3(ratio)}}};
}

std::vector<Verdict> review(const std::vector<Tick>& ticks) {
    return {detect_antiaim(ticks), detect_aim_snap(ticks),
            detect_no_recoil(ticks), detect_bhop(ticks)};
}

}  // namespace detectors
