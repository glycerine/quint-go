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
}

def states(path):
    data = json.load(open(path))
    out = []
    for st in data['states']:
        out.append({k: value(v) for k, v in st.items() if not k.startswith('#')})
    return out

ok = True
for name, path in [(a.split('=')[0], a.split('=')[1]) for a in sys.argv[1:]]:
    init, step, inv, enabled = SPECS[name]
    trace = states(path)
    valid_init = init(trace[0])
    valid_steps = all(step(trace[i], trace[i + 1]) for i in range(len(trace) - 1))
    invariant_violated = any(not inv(s) for s in trace)
    # Deadlock witness: the trace ends where no step is enabled (the checker's
    # deadlock check). Here the final state must have no enabled transition.
    deadlocked = not enabled(trace[-1])
    violated = invariant_violated or deadlocked
    verdict = valid_init and valid_steps and violated
    ok = ok and verdict
    print(f"{name}: states={len(trace)} init={valid_init} steps={valid_steps} violated={invariant_violated} deadlock={deadlocked} valid={verdict}")
sys.exit(0 if ok else 1)
