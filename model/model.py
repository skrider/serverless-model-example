import uuid
import time 

class Model:
	def __init__(self):
		time.sleep(10)
		self.return_val = "world" + str(uuid.uuid4())

	def predict(self, hello: str):
		time.sleep(3)
		return {"output": self.return_val, "input": hello}
