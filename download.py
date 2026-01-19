import requests
import os
import sys
import re
import subprocess
import math
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path


# =========================
# COMMON HELPERS
# =========================

def fetch_json_with_retry(url, retries=5, timeout=15, delay=3):
    for attempt in range(retries):
        try:
            r = requests.get(url, timeout=timeout)
            r.raise_for_status()
            return r.json()
        except Exception as e:
            print(f"⚠️ Fetch failed ({attempt+1}/{retries}): {e}")
            if attempt < retries - 1:
                time.sleep(delay)
    print(f"❌ Failed to fetch after {retries} retries: {url}")
    return None


def download_file(url, save_path, desc):
    for attempt in range(3):
        try:
            with requests.get(url, timeout=15, stream=True) as r:
                r.raise_for_status()
                with open(save_path, "wb") as f:
                    for chunk in r.iter_content(1024 * 1024):
                        if chunk:
                            f.write(chunk)
            print(f"✅ Downloaded: {desc}")
            return True
        except Exception as e:
            print(f"⚠️ Download failed ({attempt+1}/3): {desc} - {e}")
            time.sleep(3)
    return False


# =========================
# MERGE FUNCTION
# =========================

def merge_drama(folder_path, limit):
    folder = Path(folder_path)
    title = folder.name.lower().replace(" ", "_")
    files = sorted(
        [(f, int(re.search(r'ep(\d+)', f).group(1)))
         for f in os.listdir(folder) if re.search(r'ep\d+\.mp4', f, re.I)],
        key=lambda x: x[1]
    )

    if not files:
        print("⚠️ No episode files found")
        return

    batches = math.ceil(len(files) / limit)
    files_txt = folder / "files.txt"

    for i in range(batches):
        batch = files[i * limit:(i + 1) * limit]
        output = folder / f"{title}_part_{i+1}.mp4"

        with open(files_txt, "w", encoding="utf-8") as f:
            for name, _ in batch:
                f.write(f"file '{(folder / name).absolute()}'\n")

        print(f"🚀 Merging batch {i+1}/{batches}")
        subprocess.run([
            "ffmpeg", "-y",
            "-f", "concat", "-safe", "0",
            "-i", str(files_txt),
            "-c", "copy",
            str(output)
        ],capture_output=True, text=True)

        for name, _ in batch:
            os.remove(folder / name)

    files_txt.unlink(missing_ok=True)
    print("🎉 Merge completed")


# =========================
# PLATFORM HANDLERS
# =========================

def fetch_flickreels(target_path, series_id, limit, title):
    url = f"https://api.sansekai.my.id/api/flickreels/detailAndAllEpisode?id={series_id}"
    data = fetch_json_with_retry(url)
    if not data:
        return

    episodes = data.get("episodes", [])
    cover = data.get("drama", {}).get("cover")

    process_episodes(
        target_path, title, episodes, limit,
        lambda ep, i: ep.get("raw", {}).get("videoUrl"),
        cover
    )


def fetch_dramabox(target_path, series_id, limit, title, cover_url):
    ep_url = f"https://api.sansekai.my.id/api/dramabox/allepisode?bookId={series_id}"

    episodes = fetch_json_with_retry(ep_url)

    if not episodes:
        return

    cover = cover_url

    def extract_url(ep, i):
        try:
            return ep["cdnList"][1]["videoPathList"][1]["videoPath"]
        except Exception:
            return None

    process_episodes(target_path, title, episodes, limit, extract_url, cover)


# =========================
# MAIN DOWNLOAD LOGIC
# =========================

def process_episodes(base_path, title, episodes, limit, url_getter, cover_url):
    
    # 1. Setup Directory 
    slug_title = title.lower().replace(" ", "_")
    folder = Path(base_path) / title
    folder.mkdir(parents=True, exist_ok=True)

    if cover_url:
        download_file(cover_url, folder / f"cover_{slug_title}.jpg", "Cover")

    with ThreadPoolExecutor(max_workers=5) as pool:
        tasks = []
        for i, ep in enumerate(episodes):
            url = url_getter(ep, i)
            if not url:
                continue
            file = folder / f"ep{i+1}.mp4"
            if file.exists():
                continue
            tasks.append(pool.submit(download_file, url, file, f"EP {i+1}"))

        for t in tasks:
            t.result()

    merge_drama(folder, limit)


# =========================
# ENTRY POINT
# =========================

def download_drama(path, series_id, limit, title, platform, cover_url):
    if platform == "flickreels":
        fetch_flickreels(path, series_id, limit, title)
    elif platform == "dramabox":
        fetch_dramabox(path, series_id, limit, title, cover_url)
    else:
        print("❌ Unknown platform")


if __name__ == "__main__":
    if len(sys.argv) < 4:
        print("Usage: python script.py <series_id> <title> <platform>")
        sys.exit(1)

    download_drama(
        path="home/melolo",
        series_id=sys.argv[1],
        limit=16,
        title=sys.argv[2],
        platform=sys.argv[3],
        cover_url=sys.argv[4]
    )
    # download_drama(r"C:\\Users\hp\melolo", 41000103119, 16, "Perwira Ganteng Ngebet Nikah", "dramabox")