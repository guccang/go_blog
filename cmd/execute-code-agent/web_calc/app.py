from flask import Flask, render_template, request

app = Flask(__name__)

@app.route('/')
def index():
    return render_template('index.html')

@app.route('/calculate', methods=['POST'])
def calculate():
    expression = request.form.get('expression', '')
    try:
        # Safe evaluation: only allow digits, operators, and parentheses
        allowed = set('0123456789+-*/(). ')
        if not all(ch in allowed for ch in expression):
            result = 'Invalid characters'
        else:
            # Evaluate expression
            result = eval(expression)
    except Exception as e:
        result = f'Error: {e}'
    return render_template('index.html', expression=expression, result=result)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8883, debug=True)
