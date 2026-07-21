import json

with open("fixtures/crb-patch-events.jsonl") as f:
    for i, line in enumerate(f):
        ev = json.loads(line)
        name = ev.get("objectRef", {}).get("name")
        print(f"--- event {i}: name={name} ---")
        print("requestObject:", json.dumps(ev.get("requestObject"))[:600])
        resp = ev.get("responseObject") or {}
        print("responseObject subjects:", json.dumps(resp.get("subjects")))
        print("annotations keys:", list(ev.get("annotations", {}).keys()))
        print()
