"""Offline execution of actual generated Neutron quota SQL against schema fixtures.

SQLite checks grouping and boundaries; MariaDB integration remains separate.
"""
from pathlib import Path
import re
import sqlite3
import unittest

ROOT = Path(__file__).resolve().parents[1]
RESOURCES = ('floatingip', 'router', 'network', 'port', 'security_group', 'security_group_rule', 'subnet', 'subnetpool', 'rbac_policy')
TABLES = ('floatingips', 'routers', 'networks', 'ports', 'securitygroups', 'securitygrouprules', 'subnets', 'networkrbacs', 'subnetpools')
CUTOFF = '2026-10-28 12:00:00'


class NeutronQuotaSQLTest(unittest.TestCase):
    def setUp(self):
        self.db = sqlite3.connect(':memory:')
        self.addCleanup(self.db.close)
        schema = (ROOT / 'schema/neutron/schema.sql').read_text()
        for table in (*TABLES, 'quotas', 'reservations', 'resourcedeltas'):
            ddl = re.search(r'CREATE TABLE\s+`' + table + r'`\s*\(.*?\);', schema, re.S).group()
            ddl = re.sub(r"enum\s*\([^)]*\)", 'TEXT', ddl)
            ddl = re.sub(r',?\n\s*(?:UNIQUE )?KEY[^\n]*', '', ddl)
            self.db.execute(ddl)
        source = (ROOT / 'neutron/db/queries.sql').read_text()
        generated = (ROOT / 'neutron/db/queries.sql.go').read_text()
        self.queries = {}
        for name in ('GetQuotas', 'GetResourceCountsByProject', 'GetActiveReservationsByProject'):
            sql = re.search(r'const ' + name + r' = `(.*?)\n`', generated.replace('` + "`" + `', '`'), re.S).group(1)
            sql = sql.replace('` + "`" + `', '`')
            original = source.split('-- name: ' + name + ' :many', 1)[1].split('-- name:', 1)[0]
            def normalize(s):
                return ' '.join(line.strip() for line in s.splitlines() if not line.lstrip().startswith('--')).strip().removesuffix(';')
            self.assertEqual(normalize(sql), normalize(original.replace('sqlc.arg(cutoff)', '?')))
            self.queries[name] = sql

    def reservation(self, id, owner, expiration, resource, amount):
        self.db.execute('INSERT INTO reservations VALUES (?, ?, ?)', (id, owner, expiration))
        self.db.execute('INSERT INTO resourcedeltas VALUES (?, ?, ?)', (resource, id, amount))

    def rows(self):
        return self.db.execute(self.queries['GetActiveReservationsByProject'], (CUTOFF,)).fetchall()

    def test_all_resources_and_expiration_groups(self):
        for resource in RESOURCES:
            self.reservation(resource+'1', 'owner', CUTOFF, resource, 2)
            self.reservation(resource+'2', 'owner', '2026-10-28 12:01:00', resource, 3)
            self.reservation(resource+'3', 'owner', '2026-10-28 11:59:59', resource, 100)
            self.reservation(resource+'4', 'owner', None, resource, 100)
        self.assertEqual(set(self.rows()), {('owner', resource, 5, 0) for resource in RESOURCES})

    def test_reservation_only_owner_null_amount_and_null_owner(self):
        self.reservation('active', 'reservation-only', CUTOFF, 'port', 0)
        self.reservation('bad', 'bad-owner', CUTOFF, 'router', None)
        self.reservation('null-owner', None, CUTOFF, 'port', 2)
        self.reservation('extension', 'extension-owner', CUTOFF, 'extension', 10)
        self.assertEqual(set(self.rows()), {('reservation-only', 'port', 0, 0), ('bad-owner', 'router', 0, 1)})

    def test_multiple_projects_and_negative_amounts(self):
        for resource in RESOURCES:
            self.reservation(resource+'a', 'a', CUTOFF, resource, 2)
            self.reservation(resource+'b', 'b', CUTOFF, resource, 8)
            self.reservation(resource+'c', 'a', CUTOFF, resource, 3)
        self.assertEqual(set(self.rows()), {(p, r, n, 0) for p, n in (('a', 5), ('b', 8)) for r in RESOURCES})
        self.reservation('negative', 'a', CUTOFF, 'subnet', -1)
        self.assertIn(('a', 'subnet', 4, 1), self.rows())

    def test_owning_rows_and_usage_only_projects(self):
        for i, table in enumerate(TABLES):
            # Fill required fixture columns without introducing object joins.
            values = {}
            for _, name, type, required, default, pk in self.db.execute('PRAGMA table_info('+table+')'):
                if required and default is None:
                    values[name] = 1 if 'int' in type.lower() else 'object-'+str(i)
            values.update(id='object-'+str(i), project_id='usage-only')
            columns = ','.join('`'+name+'`' for name in values)
            self.db.execute('INSERT INTO '+table+' ('+columns+') VALUES ('+','.join('?' for _ in values)+')', tuple(values.values()))
        rows = self.db.execute(self.queries['GetResourceCountsByProject']).fetchall()
        self.assertEqual(set(rows), {('usage-only', r, 1) for r in RESOURCES})

    def test_system_owned_usage_excluded_in_every_arm(self):
        for i, table in enumerate(TABLES):
            for j, owner in enumerate(('a', 'a', 'b', '', None)):
                values = {}
                for _, name, type, required, default, pk in self.db.execute('PRAGMA table_info('+table+')'):
                    if required and default is None:
                        values[name] = 1 if 'int' in type.lower() else 'object'
                values.update(id=f'object-{i}-{j}', project_id=owner)
                columns = ','.join('`'+name+'`' for name in values)
                self.db.execute('INSERT INTO '+table+' ('+columns+') VALUES ('+','.join('?' for _ in values)+')', tuple(values.values()))
        rows = self.db.execute(self.queries['GetResourceCountsByProject']).fetchall()
        self.assertEqual(set(rows), {(p, r, n) for p, n in (('a', 2), ('b', 1)) for r in RESOURCES})

    def test_system_owned_reservations_excluded(self):
        for r in RESOURCES:
            for i, (p, amount) in enumerate((('a', 2), ('b', 3), ('', 40), (None, 50))):
                self.reservation(r+str(i), p, CUTOFF, r, amount)
        self.assertEqual(set(self.rows()), {(p, r, n, 0) for p, n in (('a', 2), ('b', 3)) for r in RESOURCES})

    def test_invalid_amounts_and_signed_totals_in_every_resource(self):
        cases = {'mixed': (7, -2, None), 'negative': (-5, 2),
                 'null': (None, None), 'zero': (0,), 'large': (2147483647, 1)}
        expected = {'mixed': (5, 2), 'negative': (-3, 1),
                    'null': (0, 2), 'zero': (0, 0), 'large': (2147483648, 0)}
        for resource in RESOURCES:
            for owner, amounts in cases.items():
                for i, amount in enumerate(amounts):
                    self.reservation(f'{resource}-{owner}-{i}', owner, CUTOFF, resource, amount)
            # Invalid amounts outside the active set must not affect either aggregate.
            self.reservation(resource+'-expired', 'mixed', '2026-10-28 11:59:59', resource, -100)
            self.reservation(resource+'-no-expiry', 'mixed', None, resource, None)
        self.assertEqual(set(self.rows()), {(p, r, n, bad) for p, (n, bad) in expected.items() for r in RESOURCES})

    def test_null_cutoff_matches_nothing(self):
        self.reservation('active', 'owner', CUTOFF, 'port', 2)
        self.assertEqual(self.db.execute(self.queries['GetActiveReservationsByProject'], (None,)).fetchall(), [])

    def test_limits_preserve_zero_unlimited_null_and_duplicates(self):
        for i, value in enumerate((0, -1, None, 42)):
            self.db.execute('INSERT INTO quotas VALUES (?, ?, ?, ?)', (str(i), 'owner', 'port', value))
        rows = self.db.execute(self.queries['GetQuotas']).fetchall()
        self.assertEqual(rows, [('owner', 'port', v) for v in (0, -1, None, 42)])

    def test_limits_keep_empty_owners_and_exact_duplicates(self):
        for i, owner in enumerate(('owner', 'owner', '', None)):
            self.db.execute('INSERT INTO quotas VALUES (?, ?, ?, ?)', (str(i), owner, 'port', 42))
        rows = self.db.execute(self.queries['GetQuotas']).fetchall()
        self.assertCountEqual(rows, [('owner', 'port', 42), ('owner', 'port', 42), ('', 'port', 42)])


if __name__ == '__main__':
    unittest.main()
