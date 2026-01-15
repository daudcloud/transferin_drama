import os
import re
import time
import shutil
from concurrent.futures import ThreadPoolExecutor, as_completed

import requests
from selenium import webdriver
from selenium.webdriver.chrome.service import Service
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from webdriver_manager.chrome import ChromeDriverManager

# ===== CONFIG =====
LINKS_FILE = "video.txt"
ROOT_DOWNLOAD = os.path.abspath("Pedang pembelah ruang")
MAX_WORKERS = 5

PAGE_TIMEOUT = 25
RETRIES = 5
MIN_GOOD_SIZE = 30 * 1024   # minimum accepted size (30 KB). Increase if real videos are bigger.
SLEEP_BETWEEN = 0.5        # small pause to reduce rate-limits
# ==================

os.makedirs(ROOT_DOWNLOAD, exist_ok=True)


def read_links(path):
    with open(path, "r", encoding="utf-8") as f:
        return [ln.strip() for ln in f if ln.strip().startswith("http")]


def create_driver():
    opts = Options()
    #opts.add_argument("--headless=new")
    opts.add_argument("--no-sandbox")
    opts.add_argument("--disable-dev-shm-usage")
    # small window helps some sites layout
    opts.add_argument("--window-size=1280,900")
    driver = webdriver.Chrome(service=Service(ChromeDriverManager().install()), options=opts)
    # set a Referer header for network requests initiated by driver (helps CDNs)
    try:
        driver.execute_cdp_cmd("Network.enable", {})
        driver.execute_cdp_cmd("Network.setExtraHTTPHeaders",
                               {"headers": {"Referer": "https://getsnackvideo.com/", "User-Agent": "Mozilla/5.0"}})
    except Exception:
        pass
    return driver


def fetch_generated_href(driver, snack_url):
    driver.get("https://getsnackvideo.com")

    # cookie consent
    try:
        WebDriverWait(driver, 5).until(
            EC.element_to_be_clickable((By.XPATH, "//button[contains(., 'Do not consent')]"))
        ).click()
    except Exception:
        try:
            WebDriverWait(driver, 5).until(
                EC.element_to_be_clickable((By.XPATH, "//button[@aria-label='Close']"))
            ).click()
        except Exception:
            pass

    # input the snack link
    input_el = WebDriverWait(driver, PAGE_TIMEOUT).until(
        EC.presence_of_element_located((By.CSS_SELECTOR, "input.form-control"))
    )
    input_el.clear()
    input_el.send_keys(snack_url)

    # click primary button
    WebDriverWait(driver, PAGE_TIMEOUT).until(
        EC.element_to_be_clickable((By.CSS_SELECTOR, "button.btn-primary"))
    ).click()

    # optional close inside page
    try:
        WebDriverWait(driver, 5).until(EC.element_to_be_clickable((By.CSS_SELECTOR, "div.close-button"))).click()
    except Exception:
        pass

    # find a.btn-primary that contains an href
    a_tags = WebDriverWait(driver, PAGE_TIMEOUT).until(
        EC.presence_of_all_elements_located((By.CSS_SELECTOR, "a.btn-primary"))
    )
    for a in a_tags:
        href = a.get_attribute("href") or ""
        if href.startswith("http"):
            return href

    # fallback: scan anchors for mp4/cdn hints
    anchors = driver.find_elements(By.TAG_NAME, "a")
    for a in anchors:
        href = a.get_attribute("href") or ""
        if href.startswith("http") and (".mp4" in href or "cdn" in href):
            return href

    raise RuntimeError("No downloadable href found on page")


def build_session_from_driver(driver):
    sess = requests.Session()
    # set headers: use same UA as browser and set referer
    try:
        ua = driver.execute_script("return navigator.userAgent;")
    except Exception:
        ua = "Mozilla/5.0"
    sess.headers.update({"User-Agent": ua, "Referer": "https://getsnackvideo.com/"})
    # add cookies from Selenium
    for c in driver.get_cookies():
        # requests cookie jar expects name/value; domain/path handled automatically
        sess.cookies.set(c["name"], c["value"], domain=c.get("domain"))
    return sess


def extract_mp4_from_html_text(text):
    # common heuristics to find mp4 URL inside an HTML response
    m = re.search(r'(https?://[^"\'>\s]+\.mp4[^"\'>\s]*)', text, flags=re.IGNORECASE)
    if m:
        return m.group(1)
    m = re.search(r'src=["\'](https?://[^"\']+)["\']', text, flags=re.IGNORECASE)
    if m and (".mp4" in m.group(1) or "cdn" in m.group(1)):
        return m.group(1)
    m = re.search(r'content=["\']\d+;\s*url=(https?://[^"\']+)["\']', text, flags=re.IGNORECASE)
    if m:
        return m.group(1)
    m = re.search(r'window\.location(?:\.href)?\s*=\s*["\'](https?://[^"\']+)["\']', text, flags=re.IGNORECASE)
    if m:
        return m.group(1)
    return None


def download_via_session(sess, url, out_path, min_good_size=MIN_GOOD_SIZE, timeout=(15, 300)):
    """Download url with session to out_path. Return True if valid (>min_good_size)."""
    for attempt in range(RETRIES):
        try:
            with sess.get(url, stream=True, timeout=timeout, allow_redirects=True) as r:
                r.raise_for_status()
                ctype = r.headers.get("content-type", "").lower()
                # if server returned HTML/redirect page, try to extract mp4 link
                if "html" in ctype:
                    body = r.text
                    mp4 = extract_mp4_from_html_text(body)
                    if mp4 and mp4 != url:
                        url = mp4
                        # retry downloading new mp4 url
                        continue
                    else:
                        # received HTML and couldn't find mp4 — fail this attempt
                        return False
                # stream to temp file
                tmp = out_path + ".part"
                with open(tmp, "wb") as fw:
                    for chunk in r.iter_content(chunk_size=8192):
                        if chunk:
                            fw.write(chunk)
                size = os.path.getsize(tmp)
                if size >= min_good_size:
                    # move to final atomically
                    if os.path.exists(out_path):
                        os.remove(out_path)
                    os.replace(tmp, out_path)
                    return True
                else:
                    # too small -> delete and retry (maybe placeholder)
                    try:
                        os.remove(tmp)
                    except Exception:
                        pass
                    time.sleep(0.5 + attempt * 0.5)
        except Exception as e:
            # transient error — retry
            time.sleep(0.8 + attempt * 0.6)
            continue
    return False


def process_one(part_idx, snack_url):
    out_path = os.path.join(ROOT_DOWNLOAD, f"part_{part_idx}.mp4")
    attempt = 0
    while attempt < RETRIES:
        driver = None
        try:
            driver = create_driver()
            print(f"[Part {part_idx} | attempt {attempt+1}] loading generator page...")
            href = fetch_generated_href(driver, snack_url)
            print(f"[Part {part_idx}] generated href: {href}")
            sess = build_session_from_driver(driver)
            # small pause to reduce chance of immediate block
            time.sleep(SLEEP_BETWEEN)
            ok = download_via_session(sess, href, out_path)
            if ok:
                print(f"[Part {part_idx}] ✅ downloaded -> {out_path} ({os.path.getsize(out_path)} bytes)")
                return True
            else:
                print(f"[Part {part_idx}] ⚠️ downloaded file invalid or HTML placeholder. retrying...")
                attempt += 1
                time.sleep(1 + attempt)
        except Exception as e:
            print(f"[Part {part_idx}] error: {e}")
            attempt += 1
            time.sleep(1 + attempt)
        finally:
            try:
                if driver:
                    driver.quit()
            except Exception:
                pass
    print(f"[Part {part_idx}] ❌ failed after {RETRIES} attempts.")
    return False


def main():
    links = read_links(LINKS_FILE)
    if not links:
        print("No links found in", LINKS_FILE)
        return

    tasks = [(i, u) for i, u in enumerate(links, start=1)]
    with ThreadPoolExecutor(max_workers=MAX_WORKERS) as ex:
        futures = {ex.submit(process_one, i, url): i for i, url in tasks}
        for fut in as_completed(futures):
            idx = futures[fut]
            try:
                fut.result()
            except Exception as e:
                print(f"[Part {idx}] unexpected error in worker: {e}")

    print("All tasks finished.")


if __name__ == "__main__":
    main()
