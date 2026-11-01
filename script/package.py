from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile

root = Path(__file__).resolve().parent.parent

with ZipFile(root / "build/tagged.zip", "w", ZIP_DEFLATED) as archive:
    archive.write(root / "build/bootstrap", "bootstrap")
    for path in sorted((root / "src/web/dist").rglob("*")):
        if path.is_file():
            archive.write(path, Path("web") / path.relative_to(root / "src/web/dist"))
