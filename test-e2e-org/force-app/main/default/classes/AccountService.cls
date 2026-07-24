public with sharing class AccountService {
    // Devuelve las cuentas marcadas como activas mediante el campo custom Status__c.
    public static List<Account> activeAccounts(List<Account> accounts) {
        List<Account> result = new List<Account>();
        for (Account acc : accounts) {
            if (acc.Status__c == 'Active') {
                result.add(acc);
            }
        }
        return result;
    }
}
