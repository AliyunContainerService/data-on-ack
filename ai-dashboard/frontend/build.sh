if [ ! -d ../backend/src/main/resources/static/ ]; then
        mkdir ../backend/src/main/resources/static/
fi
npm run build:prod
ls ../backend/src/main/resources/static/
rm -rf ../backend/src/main/resources/static/*
cp -r dist/* ../backend/src/main/resources/static/
ls ../backend/src/main/resources/static/
