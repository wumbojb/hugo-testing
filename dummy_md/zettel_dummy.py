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
concepts = load_yaml("./dummy_md/placeholders/concepts.yaml")  # File baru untuk konsep zettelkasten
tags = load_yaml("./dummy_md/placeholders/tags.yaml")
lorem_paragraphs = load_yaml("./dummy_md/placeholders/lorem_paragraphs.yaml")
lorem_descriptions = load_yaml("./dummy_md/placeholders/lorem_description.yaml")

content_dir = "content/zettelkasten"
os.makedirs(content_dir, exist_ok=True)

# --- HELPER FUNCTIONS ---
def generate_zettel_content(current_id, all_ids):
    # Konten zettelkasten lebih pendek dan fokus
    content_elements = []
    
    # Paragraf pembuka
    content_elements.append(" ".join(random.sample(lorem_paragraphs, k=random.randint(1, 2))))
    
    # Beberapa poin penting
    if random.random() < 0.7:
        points = []
        for _ in range(random.randint(2, 4)):
            points.append(f"- {random.choice(lorem_descriptions)}")
        content_elements.append("\n".join(points))
    
    # Kutipan atau catatan khusus
    if random.random() < 0.4:
        content_elements.append(f"> **Catatan**: {random.choice(lorem_descriptions)}")
    
    # Kode singkat (jika relevan)
    if random.random() < 0.3:
        lang = random.choice(["python", "bash", "javascript"])
        code = f"```{lang}\n# {random.choice(title_words)}\n{random.choice(['print()', 'echo', 'console.log()'])}\n```"
        content_elements.append(code)
    
    # Referensi ke zettel lain
    if len(all_ids) > 1 and random.random() < 0.6:
        referenced_id = random.choice([id for id in all_ids if id != current_id])
        content_elements.append(f"Lihat juga: [[{referenced_id}]]")
    
    return "\n\n".join(content_elements)

# --- GENERATE ZETTELKASTEN NOTES ---
total_notes = 5  # Jumlah catatan zettelkasten
all_note_ids = []

# Generate ID unik untuk setiap catatan
for i in range(total_notes):
    # Format ID: ZETTEL-0001, ZETTEL-0002, dst.
    note_id = f"ZETTEL-{i+1:04d}"
    all_note_ids.append(note_id)

# Generate catatan
for i, note_id in enumerate(all_note_ids):
    # Judul berdasarkan konsep
    title = random.choice(concepts)
    
    # Tanggal (dalam rentang 2 tahun terakhir)
    rand_days = random.randint(0, 730)  # 2 tahun
    rand_seconds = random.randint(0, 86400)
    date = (datetime.now().astimezone() - timedelta(days=rand_days, seconds=rand_seconds))
    
    # Tag (lebih sedikit daripada post blog)
    note_tags = random.sample(tags, k=random.randint(1, 3))
    tags_yaml = [f'"{t}"' for t in note_tags]
    
    # Konten
    content = generate_zettel_content(note_id, all_note_ids)
    
    # Tautan terkait (beberapa catatan acak)
    related_notes = []
    if len(all_note_ids) > 5:
        related_notes = random.sample([id for id in all_note_ids if id != note_id], k=random.randint(6, 10))
    
    # Write markdown
    filename = f"{content_dir}/{note_id}.md"
    with open(filename, "w") as f:
        f.write(f"""---
id: "{note_id}"
title: "{title}"
date: {date.strftime('%Y-%m-%dT%H:%M:%S%z')}
tags: [{", ".join(tags_yaml)}]
---

{content}

## Tautan Terkait

""")
        
        for related_note in related_notes:
            f.write(f"- [[{related_note}]]\n")
        
        f.write(f"\n*ID: {note_id}*")

print(f"✅ Generated {total_notes} zettelkasten notes with internal linking!")

# Generate peta konsep (opsional)
with open(f"{content_dir}/00 - Zettelkasten Index.md", "w") as f:
    f.write("""---
title: "Zettelkasten Index"
date: 2023-01-01T00:00:00+0000
---

# Indeks Zettelkasten

## Konsep Utama

""")
    
    # Kelompokkan catatan berdasarkan tag
    tag_groups = {}
    for note_id in all_note_ids:
        # Dalam implementasi nyata, Anda akan membaca tag dari setiap file
        # Di sini kita hanya mensimulasikan
        simulated_tags = random.sample(tags, k=random.randint(1, 3))
        for tag in simulated_tags:
            if tag not in tag_groups:
                tag_groups[tag] = []
            tag_groups[tag].append(note_id)
    
    # Tulis kelompok tag
    for tag, notes in tag_groups.items():
        f.write(f"\n### {tag}\n")
        for note_id in random.sample(notes, k=min(5, len(notes))):
            f.write(f"- [[{note_id}]]\n")

print("✅ Generated zettelkasten index!")