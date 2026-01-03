@echo off
echo --- GENERATING SITE ---
go run .

echo.
echo --- PUSHING TO GITHUB ---
git add .
git commit -m "%*"
git push

echo.
echo --- DONE ---
pause