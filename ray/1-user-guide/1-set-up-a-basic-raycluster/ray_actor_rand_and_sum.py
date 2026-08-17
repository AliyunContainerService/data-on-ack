import random

import ray

ray.init(address="auto")

@ray.remote
class RandIntActor:
    def __init__(self, min, max):
        self.min = min
        self.max = max

    def sample(self):
        return random.randint(self.min, self.max)

@ray.remote
class AddActor:
    def __init__(self, number):
        self.number = number 

    def add(self, n):
        return n + self.number

rand_actor = RandIntActor.remote(1, 100)
add_actor = AddActor.remote(5)

rand_num = ray.get(rand_actor.sample.remote())
print(f"Random number: {rand_num}")

result = ray.get(add_actor.add.remote(rand_num))
print(f"Final result: {result}")

