"""
Python file with indentation and syntax errors.
"""

def broken_function():
    print("This function has problems")
        # Incorrect indentation - should be at same level as print
        x = 10
    # Mixing tabs and spaces
	y = 20
    
    if x > 5:
    print("This should be indented")  # Missing indentation
    
  # Inconsistent indentation level
  for i in range(10):
      print(i)
        # Too much indentation
        if i > 5:
      break  # Incorrect indentation for break

class BrokenClass:
def method_without_proper_indentation(self):  # Should be indented
    pass
    
    def another_method(self):
        try:
            result = 1 / 0
    except ZeroDivisionError:  # Missing indentation
        print("Division by zero")
        
# Function with mixed indentation
def mixed_indentation():
    x = 1
	y = 2  # Tab instead of spaces
    z = 3
	return x + y + z  # Another tab

if __name__ == "__main__":
broken_function()  # Missing indentation