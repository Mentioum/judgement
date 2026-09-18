"""Call the Go CLI from an agent-written Python script; no Python SDK needed."""
import json
import subprocess
import sys

request = {
    "state": sys.stdin.read(),
    "questions": {
        "refund": {"type": "noul", "instructions": "Is a refund requested?"}
    },
}
payload = json.dumps(request)
validation = subprocess.run(
    ["judgement", "validate"], input=payload, text=True, capture_output=True
)
if validation.returncode:
    sys.stderr.write(validation.stderr)
    sys.exit(validation.returncode)
evaluation = subprocess.run(
    ["judgement", "evaluate", "--timeout", "30s"],
    input=payload, text=True, capture_output=True,
)
if evaluation.returncode:
    sys.stderr.write(evaluation.stderr)
    sys.exit(evaluation.returncode)
result = json.loads(evaluation.stdout)
print(json.dumps({"refund_probability": result["answers"]["refund"]["noul"],
                  "model": result["model"], "usage": result["usage"]}))
