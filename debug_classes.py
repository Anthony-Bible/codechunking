from dataclasses import dataclass

@dataclass
class Dog:
    name: str
    
    def speak(self):
        return "woof"

class Cat:
    def __init__(self, name: str):
        self.name = name
        
    def speak(self):
        return "meow"