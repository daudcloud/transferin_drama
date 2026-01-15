import os
import re
import subprocess
import math

# === SETTINGS ===
folder = r"C:\Users\hp\melolo\Pilih Cinta Yang Setia" # Change to your folder path
title = folder.split('\\')[-1].lower().replace(" ", "_")
watermark_text = "telegram @DramaTrans"
merge_limit = 16  # 🔹 How many parts to merge in one batch
compress_crf = 28  # 🔹 18 = high quality, 23 = default, 28 = smaller file
compress_preset = "superfast"  # ultrafast, superfast, veryfast, fast, medium, slow

# --- Natural sort helper ---
def natural_sort_key(s):
    return [int(text) if text.isdigit() else text.lower() for text in re.split(r'(\d+)', s)]

# 1. Collect MP4 files and sort naturally
all_files = sorted(
    [f for f in os.listdir(folder) if f.lower().endswith(".mp4")],
    key=natural_sort_key
)

# 2. Filter to only "part_X" format
part_files = [(f, int(re.search(r'ep(\d+)', f).group(1))) for f in all_files if re.search(r'ep(\d+)', f)]
part_files.sort(key=lambda x: x[1])  # Sort by part number

if not part_files:
    print("⚠️ No 'epX.mp4' files found.")
    exit()

# 3. Calculate number of batches
total_parts = len(part_files)
total_batches = math.ceil(total_parts / merge_limit)

for batch_num in range(total_batches):
    start_index = batch_num * merge_limit
    end_index = min(start_index + merge_limit, total_parts)
    batch_files = [f[0] for f in part_files[start_index:end_index]]

    if not batch_files:
        continue

    batch_start_part = part_files[start_index][1]
    batch_end_part = part_files[end_index - 1][1]

    output_file = os.path.join(folder, f"{title}_part_{batch_num+1}.mp4")
    compressed_file = os.path.join(folder, f"{title}_part_{batch_num+1}_compressed.mp4")
    files_txt = os.path.join(folder, "files.txt")

    # Create files.txt for ffmpeg
    with open(files_txt, "w", encoding="utf-8") as f:
        for file in batch_files:
            f.write(f"file '{os.path.join(folder, file)}'\n")

    # Merge with watermark
    cmd_merge = [
        "ffmpeg", "-y",
        "-f", "concat", "-safe", "0", "-i", files_txt,
        "-vf", f"drawtext=text='{watermark_text}':fontcolor=white@0.3:fontsize=24:x=10:y=(h-text_h)/2:shadowcolor=black@0.3:shadowx=2:shadowy=2",
        "-c:v", "libx264", "-crf", "18", "-preset", "veryfast",
        "-c:a", "aac",
        output_file
    ]

    print(f"🚀 Merging parts {batch_start_part} to {batch_end_part}...")
    subprocess.run(cmd_merge)

    # Compress merged video
    cmd_compress = [
        "ffmpeg", "-y", "-i", output_file,
        "-vcodec", "libx264", "-crf", str(compress_crf),
        "-preset", compress_preset,
        "-acodec", "aac",
        compressed_file
    ]

    print(f"📉 Compressing {output_file}...")
    subprocess.run(cmd_compress)

    # Replace original merged file with compressed one
    os.replace(compressed_file, output_file)

    # Delete merged part files
    for file in batch_files:
        try:
            os.remove(os.path.join(folder, file))
        except Exception as e:
            print(f"⚠️ Could not delete {file}: {e}")

    # Empty files.txt
    open(files_txt, "w").close()

    print(f"✅ Batch {batch_num+1}/{total_batches} complete: {output_file}")

print("🎉 All batches merged & compressed successfully.")
