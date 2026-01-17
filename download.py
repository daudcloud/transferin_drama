import requests
import os
import sys
import re
import subprocess
import math
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

def merge_drama(final_dir, limit):
    # === SETTINGS ===
    merge_limit = int(limit)
    folder = Path(final_dir)
    title = folder.name.lower().replace(" ", "_")
    watermark_text = "telegram @DramaTrans"
    
    # Compression settings (Now applied during the merge pass)
    compress_crf = 28  
    compress_preset = "veryfast" 

    def natural_sort_key(s):
        return [int(text) if text.isdigit() else text.lower() for text in re.split(r'(\d+)', s)]

    # 1. Collect and filter files
    all_files = [f for f in os.listdir(folder) if f.lower().endswith(".mp4")]
    part_files = []
    for f in all_files:
        match = re.search(r'ep(\d+)', f, re.IGNORECASE)
        if match:
            part_files.append((f, int(match.group(1))))
    
    # Sort by episode number
    part_files.sort(key=lambda x: x[1])

    if not part_files:
        print("⚠️ No 'epX.mp4' files found.")
        return

    total_parts = len(part_files)
    total_batches = math.ceil(total_parts / merge_limit)
    files_txt = folder / "files.txt"

    try:
        for batch_num in range(total_batches):
            start_idx = batch_num * merge_limit
            end_idx = min(start_idx + merge_limit, total_parts)
            batch_data = part_files[start_idx:end_idx]
            batch_filenames = [f[0] for f in batch_data]

            output_file = folder / f"{title}_part_{batch_num+1}.mp4"

            # Create concat list
            with open(files_txt, "w", encoding="utf-8") as f:
                for filename in batch_filenames:
                    # Use absolute paths and escape single quotes for ffmpeg
                    safe_path = str((folder / filename).absolute()).replace("'", "'\\''")
                    f.write(f"file '{safe_path}'\n")

            # SINGLE PASS: Merge, Watermark, and Compress
            # We use 'complex_filter' because concat-demuxer + drawtext 
            # requires re-encoding anyway.
            cmd = [
                "ffmpeg", "-y",
                "-f", "concat", "-safe", "0", "-i", str(files_txt),
                "-c:v", "libx264", 
                "-crf", str(compress_crf), 
                "-preset", compress_preset,
                "-c:a", "aac", "-b:a", "96k", "-threads", "2", # Standardize audio bitrate
                str(output_file)
            ]

            print(f"🚀 Processing Batch {batch_num+1}/{total_batches} (EP {batch_data[0][1]}-{batch_data[-1][1]})...")
            
            result = subprocess.run(cmd, capture_output=True, text=True)
            if result.returncode != 0:
                print(f"❌ FFmpeg Error: {result.stderr}")
                continue

            # 2. Cleanup source files for this batch
            for filename in batch_filenames:
                try:
                    os.remove(folder / filename)
                except Exception as e:
                    print(f"⚠️ Could not delete {filename}: {e}")

            print(f"✅ Batch {batch_num+1} complete: {output_file.name}")

    finally:
        # Final cleanup of temp file
        if files_txt.exists():
            files_txt.unlink()
            print("🧹 Cleaned up temporary files.")

    print("🎉 All tasks finished successfully.")


def download_file(url, save_path, desc="file"):
    max_retries = 3
    retry_delay = 5
    
    for attempt in range(max_retries):
        try:
            with requests.get(url, timeout=15) as r:
                r.raise_for_status()
                with open(save_path, 'wb') as f:
                    for chunk in r.iter_content(chunk_size=1024*1024):
                        if chunk: f.write(chunk)
            print(f"✅ Success: {desc}")
            return True
        except Exception as e:
            print(f"⚠️ Attempt {attempt + 1} failed for {desc}: {e}")
            if attempt < max_retries - 1:
                time.sleep(retry_delay)
    print(f"❌ Final failure: Could not download {desc}")
    return False

def download_drama(target_path, series_id, merge_limit, title):
    api_url = f"https://api.sansekai.my.id/api/flickreels/detailAndAllEpisode?id={series_id}"
    
    print(f"Connecting to Sansekai API...")
    try:
        response = requests.get(api_url, timeout=10)
        response.raise_for_status()
        data = response.json()
    except Exception as e:
        print(f"Error fetching data: {e}")
        return

    drama_info = data.get("drama", {})
    episodes = data.get("episodes", [])
    
    # 1. Setup Directory
    # raw_title = drama_info.get("title", f"Drama_{series_id}")
    # series_title = re.sub(r'[() () ]', ' ', raw_title)
    # series_title = " ".join(series_title.split())
    series_title = title
    slug_title = series_title.lower().replace(" ", "_")
    final_dir = Path(target_path) / series_title
    final_dir.mkdir(parents=True, exist_ok=True)

    # 2. Download Cover (Synchronous)
    cover_url = drama_info.get("cover")
    if cover_url:
        download_file(cover_url, final_dir / f"cover_{slug_title}.jpg", "Cover Image")

    print(f"Series: {series_title} | Total Episodes: {len(episodes)}")
    
    # 3. Parallel Download of Episodes
    # Using 4-5 workers is usually safe; too many might trigger rate limits.
    tasks = []
    with ThreadPoolExecutor(max_workers=5) as executor:
        for ep in episodes:
            raw_data = ep.get("raw", {})
            video_url = raw_data.get("videoUrl")
            if not video_url: continue

            ep_num = ep.get("index", 0) + 1
            file_path = final_dir / f"ep{ep_num}.mp4"
            if file_path.exists():
                print(f"⏭️ Skipping ep{ep_num}.mp4 (already exists)")
                continue
            
            # Queue the download task
            tasks.append(executor.submit(download_file, video_url, file_path, f"ep{ep_num}.mp4"))

    # Wait for all downloads to finish
    for task in tasks:
        task.result()

    # 4. Call Merge function (from previous optimization)
    print("\n--- Starting Merge Process ---")
    merge_drama(str(final_dir), merge_limit)

if __name__ == "__main__":
    if len(sys.argv) >= 2:
        target_id = sys.argv[1]
        title = sys.argv[2]
        folder = "home/melolo"
        limit = 16
        download_drama(folder, target_id, limit, title)
    else:
        print("Usage: python script.py <series_id>")