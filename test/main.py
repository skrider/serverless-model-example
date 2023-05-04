import requests
import time

BASE_URL = "http://localhost:8000"
API_PUSH = BASE_URL + "/push"
API_STATUS = BASE_URL + "/status"
API_DATA = BASE_URL + "/data"
ids = []
latencies = []
outputs = []
for i in range(10):
    r = requests.post(API_PUSH, json={"input": "test"})
    _id = r.json()["id"]
    ids.append(_id)

for _id in ids:
    finished = False
    while not finished:
        r = requests.get(API_STATUS+f"/{str(_id)}")
        if r.json()["status"] == "finished":
            finished = True
        else:
            time.sleep(1)
    r = requests.get(API_DATA+f"/{str(_id)}")
    latencies.append(r.json()["latency"])
    outputs.append(r.json()["output"])

print(ids)
print(latencies)
print(outputs)
