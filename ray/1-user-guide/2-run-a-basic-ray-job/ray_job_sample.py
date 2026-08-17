import time

import ray

ray.init(address="auto")

@ray.remote
def compute_square(x: int) -> int:
    time.sleep(1)
    return x * x

@ray.remote
class SumActor:
    def __init__(self) -> None:
        self.total = 0

    def add(self, n: int) -> int:
        self.total += n
        return self.total

numbers = list(range(1, 6))
squares = ray.get([compute_square.remote(n) for n in numbers])
print(f"numbers: {numbers}")
print(f"squares: {squares}")

sum_actor = SumActor.remote()
total = ray.get(sum_actor.add.remote(sum(squares)))
print(f"total: {total}")
