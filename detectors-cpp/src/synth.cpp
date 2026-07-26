#include "synth.hpp"

#include <algorithm>
#include <cmath>

namespace synth {
namespace {

double gauss(RNG& r, double mu, double sigma) {
    std::normal_distribution<double> d(mu, sigma);
    return d(r);
}
double uniform(RNG& r, double a, double b) {
    std::uniform_real_distribution<double> d(a, b);
    return d(r);
}
int randint(RNG& r, int a, int b) {  // inclusive, like Python's random.randint
    std::uniform_int_distribution<int> d(a, b);
    return d(r);
}
double unit(RNG& r) { return uniform(r, 0.0, 1.0); }

// Minimum-jerk ramp -- the profile a hand actually produces on a flick.
std::vector<double> human_flick(RNG& r, double start_yaw, double target_yaw) {
    const double span = target_yaw - start_yaw;
    const int n = std::max(5, std::min(14, static_cast<int>(std::fabs(span) / 6) + 5));
    std::vector<double> path;
    for (int i = 1; i <= n; ++i) {
        const double s = static_cast<double>(i) / n;
        const double eased = 10 * s * s * s - 15 * s * s * s * s + 6 * s * s * s * s * s;
        path.push_back(start_yaw + span * eased + gauss(r, 0.0, 0.35));
    }
    path.push_back(target_yaw + gauss(r, 0.0, 0.8));  // overshoot correction
    return path;
}

std::vector<Tick> hop_trace(RNG& r, int n, double perfect_chance) {
    std::vector<Tick> ticks;
    int t = 0, airborne = 0;
    while (t < n) {
        if (airborne > 0) {
            const bool landing = airborne == 1;
            Tick tk;
            tk.tick = t;
            tk.on_ground = landing;
            tk.jump_pressed = landing && unit(r) < perfect_chance;
            ticks.push_back(tk);
            --airborne;
        } else {
            Tick tk;
            tk.tick = t;
            tk.on_ground = true;
            tk.jump_pressed = true;
            ticks.push_back(tk);
            airborne = 22;
        }
        ++t;
    }
    return ticks;
}

}  // namespace

std::vector<Tick> clean_player(RNG& r, int n) {
    std::vector<Tick> ticks;
    double yaw = 0.0, pitch = 0.0;
    int t = 0;
    while (t < n) {
        if (unit(r) < 0.08) {
            for (double y : human_flick(r, yaw, yaw + uniform(r, -60, 60))) {
                ticks.push_back({t, pitch + gauss(r, 0.0, 0.4), y, true, false, false});
                yaw = y;
                ++t;
            }
            // spray with hand compensation: pull down, fight the walk
            const int rounds = randint(r, 14, 26);
            for (int k = 0; k < rounds; ++k) {
                pitch += 0.55 - std::min(k, 9) * 0.02 + gauss(r, 0.0, 0.13);
                yaw += gauss(r, 0.0, 0.5);
                ticks.push_back({t, pitch, yaw, true, false, true});
                ++t;
            }
            pitch *= 0.4;
        } else {
            yaw += gauss(r, 0.0, 1.1);
            ticks.push_back({t, pitch + gauss(r, 0.0, 0.3), yaw, true, false, false});
            ++t;
        }
    }
    ticks.resize(std::min<std::size_t>(ticks.size(), n));
    return ticks;
}

std::vector<Tick> spinbot(RNG& r, int n) {
    std::vector<Tick> ticks;
    ticks.reserve(n);
    for (int t = 0; t < n; ++t) {
        ticks.push_back({t, uniform(r, -89, 89),
                         std::fmod(t * 137.5, 360.0) - 180.0, true, false,
                         t % 23 == 0});
    }
    return ticks;
}

std::vector<Tick> silent_aim(RNG& r, int n) {
    std::vector<Tick> ticks;
    double yaw = 0.0;
    int t = 0;
    while (t < n) {
        if (unit(r) < 0.12) {
            const double offset = uniform(r, -70, 70);
            ticks.push_back({t, 0.0, yaw + offset, true, false, true});  // snap + shoot
            ticks.push_back({t + 1, 0.0, yaw, true, false, false});      // and back
            t += 2;
        } else {
            yaw += gauss(r, 0.0, 1.0);
            ticks.push_back({t, gauss(r, 0.0, 0.3), yaw, true, false, false});
            ++t;
        }
    }
    ticks.resize(std::min<std::size_t>(ticks.size(), n));
    return ticks;
}

std::vector<Tick> no_recoil(RNG& r, int n) {
    std::vector<Tick> ticks;
    double yaw = 0.0;
    int t = 0;
    while (t < n) {
        const int spray = randint(r, 18, 28);
        for (int k = 0; k < spray; ++k) {
            ticks.push_back({t, gauss(r, 0.0, 0.01), yaw + gauss(r, 0.0, 0.02),
                             true, false, true});
            ++t;
        }
        const int gap = randint(r, 20, 40);
        for (int k = 0; k < gap; ++k) {
            yaw += gauss(r, 0.0, 1.0);
            ticks.push_back({t, 0.0, yaw, true, false, false});
            ++t;
        }
    }
    ticks.resize(std::min<std::size_t>(ticks.size(), n));
    return ticks;
}

std::vector<Tick> bhop_human(RNG& r, int n) { return hop_trace(r, n, 0.35); }
std::vector<Tick> bhop_script(RNG& r, int n) { return hop_trace(r, n, 1.0); }

}  // namespace synth
