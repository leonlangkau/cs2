#include <iomanip>
#include <iostream>
#include <map>
#include <string>
#include <utility>
#include <vector>

#include "detectors/detectors.hpp"
#include "synth.hpp"

namespace {

using detectors::Tick;
using detectors::Verdict;

// A fixed seed keeps runs reproducible. Date.now()-style entropy is
// deliberately avoided so `--check` is deterministic in CI.
constexpr unsigned kSeed = 0xC5;

std::string cell(const Verdict& v) {
    std::ostringstream os;
    os << (v.flagged() ? "FLAG" : "") << ' ' << std::fixed
       << std::setprecision(2) << v.score;
    std::string s = os.str();
    if (s.size() < 13) s = std::string(13 - s.size(), ' ') + s;
    return s;
}

std::map<std::string, Verdict> review_map(const std::vector<Tick>& ticks) {
    std::map<std::string, Verdict> m;
    for (const Verdict& v : detectors::review(ticks)) m[v.name] = v;
    return m;
}

}  // namespace

int main(int argc, char** argv) {
    const bool check = argc > 1 && std::string(argv[1]) == "--check";

    std::mt19937 rng(kSeed);
    // Build in a fixed order so the RNG stream -- and thus the table -- is
    // stable across runs.
    std::vector<std::pair<std::string, std::vector<Tick>>> traces;
    traces.emplace_back("clean player", synth::clean_player(rng));
    traces.emplace_back("spinbot", synth::spinbot(rng));
    traces.emplace_back("silent aim", synth::silent_aim(rng));
    traces.emplace_back("no recoil", synth::no_recoil(rng));
    traces.emplace_back("bhop (human)", synth::bhop_human(rng));
    traces.emplace_back("bhop (scripted)", synth::bhop_script(rng));

    const std::vector<std::string> names = {"anti-aim", "aim-snap", "no-recoil",
                                            "bhop-script"};

    std::cout << std::left << std::setw(18) << "trace";
    for (const std::string& n : names)
        std::cout << std::right << std::setw(13) << n;
    std::cout << "\n" << std::string(18 + 13 * names.size(), '-') << "\n";

    std::vector<std::pair<std::string, Verdict>> fired;
    for (auto& [label, ticks] : traces) {
        const std::map<std::string, Verdict> v = review_map(ticks);
        std::cout << std::left << std::setw(18) << label;
        for (const std::string& n : names)
            std::cout << cell(v.at(n));
        std::cout << "\n";
        for (const std::string& n : names)
            if (v.at(n).flagged()) fired.emplace_back(label, v.at(n));
    }

    std::cout << "\nwhy each flag fired:\n";
    for (const auto& [label, v] : fired)
        std::cout << "  " << std::left << std::setw(16) << label << std::setw(12)
                  << v.name << v.detail << "\n";

    // Spread check runs off impact positions, not the tick stream.
    std::cout << "\nspread check (stationary ak47 spray at 800 units):\n";
    std::vector<std::pair<double, double>> honest, cheating;
    std::normal_distribution<double> wide(0.0, 5.6), tight(0.0, 0.7);
    for (int i = 0; i < 24; ++i) {
        honest.emplace_back(wide(rng), wide(rng));
        cheating.emplace_back(tight(rng), tight(rng));
    }
    const Verdict honest_v = detectors::detect_no_spread("ak47", honest, 800.0);
    const Verdict cheat_v = detectors::detect_no_spread("ak47", cheating, 800.0);
    for (const auto& [tag, v] :
         {std::pair<std::string, Verdict>{"honest", honest_v},
          std::pair<std::string, Verdict>{"spread removed", cheat_v}}) {
        std::cout << "  " << (v.flagged() ? "FLAG" : "    ") << ' '
                  << std::left << std::setw(16) << tag << std::fixed
                  << std::setprecision(2) << v.score << "  " << v.detail << "\n";
    }

    if (!check) return 0;

    // --check: assert every planted cheat trips its own detector, the two
    // clean traces stay quiet, and the spread test separates. Nonzero exit on
    // any miss so CTest fails loudly.
    struct Expect {
        std::string trace, detector;
        bool want_flag;
    };
    const std::vector<Expect> expectations = {
        {"clean player", "anti-aim", false}, {"clean player", "aim-snap", false},
        {"clean player", "no-recoil", false}, {"clean player", "bhop-script", false},
        {"spinbot", "anti-aim", true},        {"silent aim", "aim-snap", true},
        {"no recoil", "no-recoil", true},     {"bhop (scripted)", "bhop-script", true},
        {"bhop (human)", "bhop-script", false},
    };

    std::map<std::string, std::map<std::string, Verdict>> byTrace;
    for (auto& [label, ticks] : traces) byTrace[label] = review_map(ticks);

    int failures = 0;
    for (const Expect& e : expectations) {
        const bool got = byTrace.at(e.trace).at(e.detector).flagged();
        if (got != e.want_flag) {
            std::cerr << "FAIL: " << e.trace << " / " << e.detector << " expected "
                      << (e.want_flag ? "FLAG" : "clean") << ", got "
                      << (got ? "FLAG" : "clean") << "\n";
            ++failures;
        }
    }
    if (honest_v.flagged()) {
        std::cerr << "FAIL: honest spray flagged as no-spread\n";
        ++failures;
    }
    if (!cheat_v.flagged()) {
        std::cerr << "FAIL: spread-removed spray not flagged\n";
        ++failures;
    }

    std::cout << "\n" << (failures ? "CHECK FAILED" : "all signature checks passed")
              << "\n";
    return failures ? 1 : 0;
}
