CREATE EXTENSION IF NOT EXISTS libx509pq;
CREATE EXTENSION pgcrypto;

CREATE TABLE certificate (
	id bigserial,
	certificate bytea NOT NULL
);

CREATE INDEX certificate_id_idx ON certificate (id);
CREATE INDEX certificate_sha1_idx ON certificate (digest(certificate, 'sha1'));
CREATE UNIQUE INDEX certificate_sha256_idx ON certificate (digest(certificate, 'sha256'));
CREATE INDEX certificate_serial_idx ON certificate (x509_serialNumber(CERTIFICATE));
CREATE INDEX certificate_spki_sha1_idx ON certificate (digest(x509_publicKey(CERTIFICATE), 'sha1'));
CREATE INDEX certificate_spki_sha256_idx ON certificate (digest(x509_publicKey(CERTIFICATE), 'sha256'));
CREATE INDEX certificate_notbefore_idx ON certificate (x509_notBefore(CERTIFICATE));
CREATE INDEX certificate_notafter_idx ON certificate (
	coalesce(
		x509_notAfter(CERTIFICATE),
		'infinity'::timestamp
	)
);
CREATE INDEX certificate_subject_sha1_idx ON certificate (digest(x509_name(CERTIFICATE), 'sha1'));
CREATE INDEX certificate_ski_idx ON certificate (x509_subjectKeyIdentifier(CERTIFICATE));

CREATE TEXT SEARCH DICTIONARY certwatch (TEMPLATE = pg_catalog.simple);
CREATE TEXT SEARCH CONFIGURATION certwatch (COPY = pg_catalog.simple);

CREATE OR REPLACE FUNCTION identities(
	cert					bytea,
	is_subject				boolean		DEFAULT true
) RETURNS tsvector
AS $$
DECLARE
	t_string				text := '';
	t_position				integer;
	t_doReverse				boolean;
	l_identity				RECORD;
BEGIN
	FOR l_identity IN (
		SELECT lower(sub.VALUE) AS IDENTITY,
				CASE WHEN sub.TYPE IN ('2.5.4.3', 'type2') THEN lower(sub.VALUE)															-- commonName, dNSName.
					WHEN sub.TYPE IN ('1.2.840.113549.1.9.1', 'type1') THEN lower(substring(sub.VALUE FROM position('@' IN sub.VALUE) + 1))	-- emailAddress, rfc822Name.
				END AS DOMAIN_NAME
			FROM (
				SELECT encode(RAW_VALUE, 'escape') AS VALUE,
						ATTRIBUTE_OID AS TYPE
					FROM public.x509_nameAttributes_raw(cert, is_subject)
				UNION
				SELECT encode(RAW_VALUE, 'escape') AS VALUE,
						('type' || TYPE_NUM::text) AS TYPE
					FROM public.x509_altNames_raw(cert, is_subject)
			) sub
			GROUP BY IDENTITY, DOMAIN_NAME
			ORDER BY LENGTH(lower(sub.VALUE)) DESC
	) LOOP
		t_string := t_string || ' ' || l_identity.IDENTITY;
		IF coalesce(l_identity.DOMAIN_NAME, '') = '' THEN
			t_string := t_string || ' ' || reverse(l_identity.IDENTITY);
		ELSE
			IF l_identity.DOMAIN_NAME != l_identity.IDENTITY THEN
				t_string := t_string || ' ' || l_identity.DOMAIN_NAME || ' ' || reverse(l_identity.IDENTITY);
			END IF;

			t_doReverse := TRUE;
			LOOP
				t_position := coalesce(position('.' IN l_identity.DOMAIN_NAME), 0);
				EXIT WHEN t_position = 0;
				l_identity.DOMAIN_NAME := substring(l_identity.DOMAIN_NAME FROM (t_position + 1));
				t_string := t_string || ' ' || l_identity.DOMAIN_NAME;
				IF t_doReverse THEN
					IF position((reverse(l_identity.DOMAIN_NAME) || '.') in t_string) = 0 THEN
						t_string := t_string || ' ' || reverse(l_identity.DOMAIN_NAME);
					END IF;
					t_doReverse := FALSE;
				END IF;
			END LOOP;
		END IF;
	END LOOP;

	RETURN strip(to_tsvector('public.certwatch', ltrim(t_string)));
END;
$$ LANGUAGE plpgsql STRICT IMMUTABLE;

CREATE INDEX certificate_identities_idx ON certificate USING GIN (identities(CERTIFICATE));
