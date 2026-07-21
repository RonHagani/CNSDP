import json
import glob

sources = [
    ("fixtures/exec-tty-true.jsonl", "pods_exec"),
    ("fixtures/pod-create-highrisk.jsonl", "high_risk_pod_create"),
    ("fixtures/crb-create-events.jsonl", "crb_create"),
    ("fixtures/crb-subject-addition-variants.jsonl", "crb_subject_add_variant"),
]

items = []
for path, _label in sources:
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            items.append(json.loads(line))

event_list = {
    "kind": "EventList",
    "apiVersion": "audit.k8s.io/v1",
    "items": items,
}

with open("fixtures/webhook-eventlist.json", "w") as f:
    json.dump(event_list, f, indent=2)

print(f"Wrote {len(items)} real captured events into fixtures/webhook-eventlist.json")
