import os
from pathlib import Path

def change_file_extensions(directory_path, old_extension, new_extension):
    """
    Changes the extension of all files in a directory that match the old extension.
    
    Args:
        directory_path (str): The path to the folder containing the files.
        old_extension (str): The current extension (e.g., '.txt').
        new_extension (str): The desired new extension (e.g., '.md').
    """
    # Ensure extensions start with a dot
    if not old_extension.startswith('.'):
        old_extension = '.' + old_extension
    if not new_extension.startswith('.'):
        new_extension = '.' + new_extension

    path = Path(directory_path)
    
    # Iterate over all items in the directory
    for file_path in path.iterdir():
        # Check if the item is a file and matches the old extension
        if file_path.is_file() and file_path.suffix == old_extension:
            new_file_path = file_path.with_suffix(new_extension)
            try:
                # Rename the file
                file_path.rename(new_file_path)
                print(f"Renamed: {file_path.name} -> {new_file_path.name}")
            except FileExistsError:
                print(f"Error: {new_file_path.name} already exists, skipping rename.")

# --- Example Usage ---
# Replace with the path to your folder, old extension, and new extension
folder_to_process = r'C:\users\hp\melolo' 
current_ext = '.mdl' 
target_ext = '.mp4' 

change_file_extensions(folder_to_process, current_ext, target_ext)
