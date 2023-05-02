from flask import Flask, request

from model import Model

app = Flask(__name__)

print("Loading model...")
model = Model()

@app.route('/', methods=['POST'])
def predict():
    # get the "input" field from the request body
    input = request.json['input']
    return model.predict(input)

@app.route('/ok')
def ok():
    return 'ok'

if __name__ == '__main__':
	app.run(host='0.0.0.0', port=8000)
