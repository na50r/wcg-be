import json

# Generates Combinations.csv and Words.csv from items.json

def generate_combinations(path):
    data = json.load(open(path))
    element_depth = {k: v['depth']
                     for k, v in data.items() if ',' not in k and k != "undefined"}
    print(f"Found {len(element_depth)} elements without comma")

    combinations = []
    for element, v in data.items():
        if element not in element_depth:
            continue
        recipes = v.get('recipes', [])
        for recipe in recipes:
            for o in recipes:
                if o['item_1'] in element_depth and o['item_2'] in element_depth:
                    combinations.append((o['item_1'], o['item_2'], element))

    recipes = {}
    for a, b, c in combinations:
        if c not in recipes:
            recipes[c] = []
        recipes[c].append((a, b))

    combination_depth = {(a, b, c): max(
        element_depth[a], element_depth[b])+1 for a, b, c in combinations}
    print(f"Found {len(combination_depth)} recipes")

    reachability = {}
    for element in element_depth:
        recipe = recipes.get(element, [])
        reach = 0
        old_depth = 999
        if recipe:
            for a, b in recipe:
                newWeight, oldWeight = 0.75, 0.25
                if old_depth > combination_depth[(a, b, element)]:
                    old_depth = combination_depth[(a, b, element)]
                else:
                    newWeight, oldWeight = 0.25, 0.75
                reach =  (1.0 / 2 ** (combination_depth[(a, b, element)])) * newWeight + reach * oldWeight
        reachability[element] = reach
    return element_depth, reachability, combination_depth


def write_combinations(out_path, combination_depth):
    with open(out_path, "w") as f:
        f.write("Depth,A,B,C\n")
        for (a, b, c), depth in combination_depth.items():
            f.write(f"{depth},{a},{b},{c}\n")


def write_reachability(out_path, reachability, element_depth):
    with open(out_path, "w") as f:
        f.write("Element,Depth,Reachability\n")
        for element, reach in reachability.items():
            f.write(f"{element},{element_depth[element]},{reach}\n")


if __name__ == "__main__":
    element_depth, reachability, combination_depth = generate_combinations(
        "items.json")
    write_combinations("Combinations.csv", combination_depth)
    write_reachability("Words.csv", reachability, element_depth)