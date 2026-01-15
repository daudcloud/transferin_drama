import gspread
from oauth2client.service_account import ServiceAccountCredentials
import requests
import time
import string

scope = ["https://spreadsheets.google.com/feeds", "https://www.googleapis.com/auth/drive"]
creds = ServiceAccountCredentials.from_json_keyfile_name("service-account.json", scope)
client = gspread.authorize(creds)
sheet = client.open("Drama_Collection").sheet1

def collect_drama():
    keywords = list(string.ascii_lowercase) + ["ceo", "nikah", "cinta", "rahasia", "istri", "presdir", "tuan muda", "miliarder", "kaya", "bos", "direktur", "penguasa",
    "suami", "menikah", "cerai", "menantu", "nenek", "keluarga", "kembali", "balas dendam", "bangkit", "hina", "menyamar", "identitas",
    "an", "ter", "ber", "si", "me", "ka", "di"]
    existing_ids = set(sheet.col_values(2))
    print(f"Loaded {len(existing_ids)} existing IDs from sheet.")
    for word in keywords:
        print(f"Searching keyword: {word}...")
        url = f"https://api.sansekai.my.id/api/flickreels/search?query={word}"
        try:
            response = requests.get(url).json()
            dramas = response.get('data', [])
            batch_to_add = []
            for d in dramas:
                d_id = str(d.get('playlet_id'))
                title = d.get('title')
                cover = d.get('cover')
                if d_id not in existing_ids:
                    batch_to_add.append([title, d_id, cover, "Pending"])
                    existing_ids.add(d_id)
            if len(batch_to_add) > 0:
                sheet.append_rows(batch_to_add, value_input_option='RAW')
                print(f" --> Successfully bulk added {len(batch_to_add)} new dramas")
            else:
                print(f" --> No new dramas found for '{word}'.")
            time.sleep(4)
        except Exception as e:
            print(f"Error searching {word} : {e}")

if __name__ == "__main__":
    collect_drama()