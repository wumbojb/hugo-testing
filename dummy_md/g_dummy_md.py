import os
import random
import re
import unicodedata
from datetime import datetime, timedelta
import yaml

# --- UTILITIES ---
def slugify(text):
    text = unicodedata.normalize('NFKD', text).encode('ascii', 'ignore').decode('ascii')
    text = text.lower()
    text = re.sub(r'[^a-z0-9\s-]', '', text)
    text = re.sub(r'[\s]+', '-', text)
    return text.strip('-')

def load_yaml(file):
    with open(file, "r") as f:
        return yaml.safe_load(f)

# --- LOAD PLACEHOLDERS ---
title_words = load_yaml("./dummy_md/placeholders/title_words.yaml")
categories = load_yaml("./dummy_md/placeholders/categories.yaml")
tags = load_yaml("./dummy_md/placeholders/tags.yaml")
lorem_paragraphs = load_yaml("./dummy_md/placeholders/lorem_paragraphs.yaml")
lorem_descriptions = load_yaml("./dummy_md/placeholders/lorem_description.yaml")

content_dir = "content/dummy"
os.makedirs(content_dir, exist_ok=True)

# --- HELPER FUNCTIONS ---
def generate_heading():
    return f"{'#' * random.choice([2,3,4])} {random.choice(title_words)}"

def generate_list():
    return "\n".join(f"- {random.choice(title_words)}" for _ in range(random.randint(3,6)))

def generate_code_block():
    snippets = {
        "python": "print('Hello World')",
        "bash": "echo Hello World",
        "javascript": "console.log('Hello World');",
        "go": 'package main\nimport "fmt"\nfunc main() { fmt.Println("Hello World") }',
        "c": '#include <stdio.h>\nint main(){ printf("Hello World"); return 0; }'
    }
    lang = random.choice(list(snippets.keys()))
    return f"```{lang}\n{snippets[lang]}\n```"

def generate_table():
    table = "| Name | Value |\n|------|-------|"
    for _ in range(random.randint(3,5)):
        table += f"\n| {random.choice(title_words)} | {random.randint(1,100)} |"
    return table

def generate_image():
    if random.random() < 0.5:
        return f"![Placeholder](https://placehold.co/{random.randint(200,600)}x{random.randint(200,400)})"
    else:
        query = random.choice(["nature", "tech", "city", "abstract", "people"])
        return f"![Unsplash](https://source.unsplash.com/random/400x300/?{query})"

def generate_blockquote():
    choices = [
        random.choice(lorem_paragraphs),
        f"Tip: {random.choice(title_words)}",
        f"Note: {random.choice(lorem_descriptions)}"
    ]
    return "> " + random.choice(choices)

def generate_inline_link():
    text = random.choice(title_words)
    url = random.choice([
        "https://example.com",
        "https://wikipedia.org",
        "https://github.com",
        "https://python.org",
        "https://golang.org"
    ])
    return f"[{text}]({url})"

def generate_paragraph(current_slug, all_slugs):
    para = []
    # Kombinasi paragraf panjang & pendek
    for _ in range(random.randint(2,5)):
        chunk = " ".join(random.sample(lorem_paragraphs, k=random.randint(1,3)))
        para.append(chunk)

    # Random insert markdown elements
    elements = [
        (0.5, generate_heading),
        (0.4, lambda: f"**{random.choice(title_words)}**"),
        (0.3, lambda: f"*{random.choice(title_words)}*"),
        (0.25, lambda: "`example_code()`"),
        (0.2, generate_code_block),
        (0.3, generate_list),
        (0.25, generate_image),
        (0.15, generate_table),
        (0.2, generate_blockquote),
        (0.25, generate_inline_link),
        (20, lambda: f"[[{random.choice([s for s in all_slugs if s != current_slug])}]]" if len(all_slugs) > 1 else "")
    ]
    for prob, func in elements:
        if random.random() < prob:
            para.append(func())
    return "\n\n".join(para)

# --- GENERATE POSTS ---
total_posts = 50000
all_slugs = []

for i in range(1, total_posts+1):
    # Random date
    rand_days = random.randint(0, 365)
    rand_seconds = random.randint(0, 86400)
    date = (datetime.now().astimezone() - timedelta(days=rand_days, seconds=rand_seconds))

    # Title & slug
    title = " ".join(random.sample(title_words, k=random.randint(3,5)))
    slug = slugify(title)
    while slug in all_slugs:
        slug += f"-{random.randint(1000,9999)}"
    all_slugs.append(slug)

    # Category & tags
    category = random.choice(categories)
    post_tags = random.sample(tags, k=random.randint(2,4))
    tags_yaml = [f'"{t}"' for t in post_tags]

    # Body
    body = "\n\n".join(generate_paragraph(slug, all_slugs) for _ in range(random.randint(8,15)))

    # Description
    description = " ".join(random.sample(lorem_descriptions, k=random.randint(2,6)))

    # Write markdown
    filename = f"{content_dir}/{slug}.md"
    with open(filename, "w") as f:
        f.write(f"""---
title: "{title}"
date: {date.strftime('%Y-%m-%dT%H:%M:%S%z')}
description: "{description}"
categories: ["{category}"]
tags: [{", ".join(tags_yaml)}]
images: ["https://placehold.co/120x320"]
draft: false
---

{body}
""")

print(f"✅ Generated {total_posts} long-form realistic dummy markdown posts with inline links!")
