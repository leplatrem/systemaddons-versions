const KINTO_URL = "https://kinto-ota.dev.mozaws.net/v1";

async function fetchVersions() {
  const url = `${KINTO_URL}/buckets/systemaddons/collections/versions/records?_sort=release.version`;
  const resp = await fetch(url);
  const body = await resp.json();
  return body.data;
}

function render(mount, versions) {
  const tpl = document.getElementById("version-info-tpl");
  mount.innerHTML = "";
  versions.forEach((version) => {
    const infos = tpl.content.cloneNode(true);
    const title = `Firefox ${version.release.version} ${version.release.target}`
    infos.querySelector(".title").textContent = title;
    infos.querySelector(".url dd").textContent = version.release.url;
    infos.querySelector(".buildId dd").textContent = version.release.buildId;
    infos.querySelector(".target dd").textContent = version.release.target;
    infos.querySelector(".lang dd").textContent = version.release.lang;
    infos.querySelector(".channel dd").textContent = version.release.channel;

    // Merge both system addons lists into one
    const builtins = (version.builtins || []).reduce((acc, addon) => {
      acc[addon.id] = {builtin: addon.version};
      return acc;
    }, {});
    const addons = (version.updates || []).reduce((acc, addon) => {
      if (addon.id in acc) {
        acc[addon.id].update = addon.version;
      } else {
        acc[addon.id] = {update: addon.version};
      }
      return acc;
    }, builtins);
    const table = infos.querySelector(".addons tbody");
    const rowTpl = document.getElementById("addon-row-tpl");
    Object.keys(addons).sort().forEach((addon) => {
      const row = rowTpl.content.cloneNode(true);
      row.querySelector(".id").textContent = addon
      row.querySelector(".builtin").textContent = addons[addon].builtin
      row.querySelector(".updated").textContent = addons[addon].update
      table.appendChild(row);
    });

    mount.appendChild(infos);
  })
}

async function main() {
  const versions = await fetchVersions();
  const mount = document.getElementById("main");
  render(mount, versions);
}

window.addEventListener("DOMContentLoaded", main);
