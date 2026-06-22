// displayName : transforme un login établissement « prenom.nom@domaine » en
// « Prénom Nom » lisible. Laisse tout le reste tel quel (logins GitHub, noms
// déjà saisis, valeurs non-email). N'altère JAMAIS l'identifiant sous-jacent
// (qui reste l'email/login = clé de jointure + id nbgrader) : usage AFFICHAGE seulement.
export function displayName(login: string | null | undefined): string {
  const s = (login ?? '').trim();
  if (!s) return '';
  // Doit ressembler à un email simple « local@domaine ».
  const at = s.indexOf('@');
  if (at <= 0) return s;
  const local = s.slice(0, at);
  // On ne « jolifie » que les locaux du type prenom.nom (lettres/.-_, séparés).
  if (!/^[a-zA-ZÀ-ÿ0-9]+([._-][a-zA-ZÀ-ÿ0-9]+)+$/.test(local)) return s;
  const cap = (w: string) => (w ? w.charAt(0).toUpperCase() + w.slice(1) : w);
  return local
    .split(/[._-]+/)
    .filter(Boolean)
    .map((part) => part.split(/(?<=\p{L})'(?=\p{L})/u).map(cap).join("'")) // gère o'connor → O'Connor
    .map(cap)
    .join(' ');
}
