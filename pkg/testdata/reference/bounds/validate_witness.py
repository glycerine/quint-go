# Independent witness check: spec semantics are restated here by hand, not taken
# from the Go checker. A witness is valid when init holds in the first state, each
# step is a legal transition, and the invariant is false in some state.
import json, sys

def value(v):
    if isinstance(v, dict) and '#bigint' in v:
        return int(v['#bigint'])
    return v

SPECS = {
    # module: (init(s), step(s, t), inv(s))
    'counter3': (lambda s: s['x'] == 0,
                 lambda s, t: t['x'] == s['x'] + 1,
                 lambda s: s['x'] < 3,
                 lambda s: s['x'] < 3),
    'jump':     (lambda s: s['x'] == 1,
                 lambda s, t: s['x'] < 100 and t['x'] == s['x'] * 2,
                 lambda s: s['x'] != 8,
                 lambda s: s['x'] < 100),
    'twovar':   (lambda s: s['x'] == 0 and s['y'] == 0,
                 lambda s, t: t['x'] == s['x'] + 1 and t['y'] == s['y'] + 2,
                 lambda s: s['x'] + s['y'] != 9,
                 lambda s: True),
    'guard':     (lambda s: s['x'] == 0,
                 lambda s, t: s['x'] < 5 and t['x'] == s['x'] + 1,
                 lambda s: s['x'] != 4,
                 lambda s: s['x'] < 5),
    'sum':       (lambda s: s['x'] == 0 and s['y'] == 0,
                 lambda s, t: t['x'] == s['x'] + 1 and t['y'] == s['y'] + 1,
                 lambda s: s['x'] + s['y'] < 10,
                 lambda s: True),
    'tlcCounter': (lambda s: s['counter'] == 0,
                 lambda s, t: s['counter'] < 3 and t['counter'] == s['counter'] + 1,
                 lambda s: s['counter'] >= 0,
                 lambda s: s['counter'] < 3),
    'toggle':   (lambda s: s['b'] is False,
                 lambda s, t: t['b'] == (not s['b']),
                 lambda s: not s['b'],
                 lambda s: True),
    'safe':     (lambda s: s['x'] == 0,
                 lambda s, t: t['x'] == (s['x'] + 1) % 4,
                 lambda s: 0 <= s['x'] < 4,
                 lambda s: True),
    # Lasso spec: x cycles 1 -> 2 -> 1 forever after 0 -> 1; y is constant.
    'cycle':    (lambda s: s['x'] == 0 and s['y'] == 0,
                 lambda s, t: t['y'] == s['y'] and (
                     (s['x'] == 0 and t['x'] == 1) or (s['x'] == 1 and t['x'] == 2) or
                     (s['x'] == 2 and t['x'] == 1)),
                 lambda s: True,
                 lambda s: s['x'] in (0, 1, 2)),
}

# Lasso witnesses violate a liveness property: the cycle states must all avoid the
# property target. For the 'cycle' spec the target is eventually(x == 5).
LASSO_AVOIDS = {'cycle': lambda s: s['x'] != 5}

def states(path):
    data = json.load(open(path))
    out = []
    for st in data['states']:
        out.append({k: value(v) for k, v in st.items() if not k.startswith('#')})
    return out, data.get('loop')

ok = True
for name, path in [(a.split('=')[0], a.split('=')[1]) for a in sys.argv[1:]]:
    init, step, inv, enabled = SPECS[name]
    trace, loop = states(path)
    valid_init = init(trace[0])
    valid_steps = all(step(trace[i], trace[i + 1]) for i in range(len(trace) - 1))
    # Lasso witness: the final transition must be a legal step back to states[loop].
    # Equality of the final state with states[loop] is not required; the successor
    # of the final state is states[loop].
    if loop is not None:
        valid_steps = valid_steps and 0 <= loop < len(trace) and step(trace[-1], trace[loop])
    invariant_violated = any(not inv(s) for s in trace)
    # Deadlock witness: the trace ends where no step is enabled (the checker's
    # deadlock check). Here the final state must have no enabled transition.
    deadlocked = not enabled(trace[-1])
    violated = invariant_violated or deadlocked
    if loop is not None:
        # The cycle from loop to the end must never reach the liveness target.
        avoids = LASSO_AVOIDS.get(name)
        violated = violated or (avoids is not None and all(avoids(s) for s in trace[loop:]))
    verdict = valid_init and valid_steps and violated
    ok = ok and verdict
    print(f"{name}: states={len(trace)} loop={loop} init={valid_init} steps={valid_steps} violated={invariant_violated} deadlock={deadlocked} valid={verdict}")
sys.exit(0 if ok else 1)
