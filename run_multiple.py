import os
import subprocess
import multiprocessing
import glob
from tqdm import tqdm

# Configuration
PARAMS_DIR = "params"  # Directory containing your param files
GO_SCRIPT = "go_delegation_queue.go"
FILE_PATTERN = "*.json" # Change extension if needed (e.g., *.txt)
MAX_WORKERS = multiprocessing.cpu_count() # Adjust based on your CPU cores

def run_go_program(param_file):
    """Runs the go command for a single parameter file."""
    cmd = ["go", "run", GO_SCRIPT, param_file]
    try:
        # capture_output=True allows you to process stdout/stderr if needed
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        return f"Success: {param_file}\nOutput: {result.stdout}"
    except subprocess.CalledProcessError as e:
        return f"Error running {param_file}: {e.stderr}"

def main():
    # Ensure the params directory exists
    if not os.path.exists(PARAMS_DIR):
        print(f"Directory '{PARAMS_DIR}' not found.")
        return

    # Get list of all param files
    param_files = glob.glob(os.path.join(PARAMS_DIR, FILE_PATTERN))
    
    if not param_files:
        print(f"No files found matching {FILE_PATTERN} in {PARAMS_DIR}")
        return

    print(f"Found {len(param_files)} files to process.")

    # Use multiprocessing.Pool for parallel execution
    with multiprocessing.Pool(processes=MAX_WORKERS) as pool:
        # imap_unordered yields results as soon as they are ready
        for result in tqdm(pool.imap_unordered(run_go_program, param_files), total=len(param_files)):
            tqdm.write(result)

if __name__ == "__main__":
    main()
