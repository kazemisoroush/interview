#!/usr/bin/env python3
"""Fail if a synthesized template contains a resource reference cycle.

cdk synth does not catch these: it synthesized the template that made the client's
callback point at the function URL while the function read the client's id, and
reported no error. CloudFormation rejected it at change set creation instead, which
is after a merge and after a deploy has already started.

    python3 no-cycles.py cdk.out/InterviewStack.template.json
"""
import collections
import json
import sys


def referenced(node, found):
    """Every logical id `node` points at, through Ref, Fn::GetAtt or DependsOn."""
    if isinstance(node, dict):
        for key, value in node.items():
            if key == "Ref" and isinstance(value, str):
                found.add(value)
            elif key == "Fn::GetAtt":
                found.add(value[0] if isinstance(value, list) else str(value).split(".")[0])
            elif key == "DependsOn":
                found.update(value if isinstance(value, list) else [value])
            else:
                referenced(value, found)
    elif isinstance(node, list):
        for item in node:
            referenced(item, found)
    return found


def cycles(resources):
    edges = {name: {d for d in referenced(body, set()) if d in resources}
             for name, body in resources.items()}
    WHITE, GREY = 0, 1
    state = collections.defaultdict(int)
    found = []

    def walk(name, path):
        state[name] = GREY
        path.append(name)
        for other in sorted(edges[name]):
            if state[other] == GREY:
                found.append(path[path.index(other):] + [other])
            elif state[other] == WHITE:
                walk(other, path)
        path.pop()
        state[name] = 2

    for name in sorted(edges):
        if state[name] == WHITE:
            walk(name, [])
    return found


def main(path):
    resources = json.load(open(path)).get("Resources", {})
    found = cycles(resources)
    for cycle in found:
        print("circular dependency: " + " -> ".join(cycle), file=sys.stderr)
    if found:
        return 1
    print(f"{path}: no cycles among {len(resources)} resources")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
