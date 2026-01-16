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

def collect_drama_box():
    keywords = ["ceo", "nikah", "cinta", "rahasia", "istri", "presdir", "tuan muda", "miliarder", "kaya", "bos", "direktur", "penguasa",
    "suami", "menikah", "cerai", "menantu", "nenek", "keluarga", "kembali", "balas dendam", "bangkit", "hina", "menyamar", "identitas",
    "an", "ter", "ber", "si", "me", "ka", "di"]
    keywords = ["konglomerat", "pewaris", "nona besar", "asisten", "sekretaris", "pelayan", "gelandangan", "miskin", "tuan putri", 
    "tunangan", "perjodohan", "mantan", "kekasih", "hamil", "anak", "kembar", "ayah", "ibu", "mertua", "pengkhianatan", "dijebak",
    "dipaksa", "kontrak", "palsu", "rebutan", "harta", "warisan", "skandal", "kaisar", "permaisuri", "selir", "pangeran", "putri", "istana", "kerajaan", "kasim", "panglima", "jenderal",
    "dewa", "dewi", "iblis", "siluman", "kultivasi", "pedang", "sakti", "abadi", "reinkarnasi", "langit", "bumi", "darah",
    "dingin", "kejam", "galak", "manis", "setia", "menderita", "tulus", "obsesi", "menemukan", "menyelamatkan", "melindungi", "mengejar", "menghilang", "menemukan", "terungkap", "membongkar",
    "sang", "sang", "kisah", "takdir", "demi", "untuk", "misteri", "ikatan", "janji", "cinta sejati", "Pewaris yang menyamar", "Balas dendam menantu", "Cinta kontrak CEO dingin", "Kembalinya kaisar pedang",
    "Istri simpanan miliarder"]
    existing_ids = set(sheet.col_values(2))
    print(f"Loaded {len(existing_ids)} existing IDs from sheet.")
    for word in keywords:
        print(f"Searching keyword: {word}...")
        url = f"https://api.sansekai.my.id/api/dramabox/search?query={word}"
        try:
            response = requests.get(url).json()
            # dramas = response.get('data', [])
            batch_to_add = []
            for d in response:
                d_id = str(d.get('bookId'))
                title = d.get('bookName')
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
    collect_drama_box()