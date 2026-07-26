cmake_minimum_required(VERSION 3.16)
project(cs2_anticheat_detectors CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

if(NOT CMAKE_BUILD_TYPE)
  set(CMAKE_BUILD_TYPE Release)
endif()

add_compile_options(-Wall -Wextra)

# Pure analysis library: per-tick state in, verdicts out. No engine, no I/O.
add_library(detectors src/detectors.cpp)
target_include_directories(detectors PUBLIC include)

# Demo binary: runs the synthetic traces through the detectors and prints the
# verdict table. `--check` turns it into a pass/fail test of the signatures.
add_executable(signatures src/main.cpp src/synth.cpp)
target_link_libraries(signatures PRIVATE detectors)

enable_testing()
add_test(NAME synthetic_signatures COMMAND signatures --check)
