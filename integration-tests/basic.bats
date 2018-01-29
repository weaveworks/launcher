#!/usr/bin/env bats

@test "basic test" {
    result="$(echo 2+2 | bc)"
    [ "$result" -eq 4 ]
}
